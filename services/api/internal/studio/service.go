// Package studio owns credential boundaries, free-provider admission and artifacts.
package studio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Lord-shaban/open-flow/services/api/internal/adapters"
	"github.com/Lord-shaban/open-flow/services/api/internal/persistence"
	"github.com/Lord-shaban/open-flow/services/api/internal/provider"
	"github.com/Lord-shaban/open-flow/services/api/internal/storage"
	"github.com/Lord-shaban/open-flow/services/api/internal/vault"
	"github.com/google/uuid"
	"strings"
	"time"
)

type Service struct {
	Store               *persistence.Store
	Vault               *vault.Vault
	Storage             *storage.S3
	OwnerID, OwnerToken string
	Adapters            map[string]provider.Adapter
}

func New(store *persistence.Store, v *vault.Vault, objects *storage.S3, owner, token, comfy string) (*Service, error) {
	s := &Service{Store: store, Vault: v, Storage: objects, OwnerID: owner, OwnerToken: token, Adapters: map[string]provider.Adapter{}}
	s.Adapters["training"] = adapters.Training{}
	s.Adapters["horde"] = adapters.NewHorde(s.Resolve)
	s.Adapters["cloudflare"] = adapters.NewCloudflare(s.Resolve)
	s.Adapters["gemini"] = adapters.NewGemini(s.Resolve)
	if comfy != "" {
		a, err := adapters.NewComfyUI(comfy)
		if err != nil {
			return nil, err
		}
		s.Adapters["comfyui"] = a
	}
	return s, nil
}
func (s *Service) Resolve(ctx context.Context, id string) (string, error) {
	c, err := s.Store.Credential(ctx, s.OwnerID, id)
	if err != nil || c.Status != "active" {
		return "", &provider.Error{Category: provider.Authentication, Message: "Active owned credential required", AcceptanceKnown: true}
	}
	value, err := s.Vault.Open(c.Ciphertext, c.Version, vault.AAD(c.OwnerID, c.Provider, c.ID, "credential"))
	if err != nil {
		return "", &provider.Error{Category: provider.Authentication, Message: "Credential cannot be decrypted", AcceptanceKnown: true}
	}
	return string(value), nil
}

type CredentialInput struct {
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	APIKey      string `json:"api_key"`
	AccountID   string `json:"account_id,omitempty"`
	WorkersFree bool   `json:"workers_free_confirmed,omitempty"`
}

func (s *Service) SaveCredential(ctx context.Context, id string, input CredentialInput) (persistence.Credential, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 120 || len(input.APIKey) < 10 || len(input.APIKey) > 4096 || strings.ContainsAny(input.APIKey, "\r\n") {
		return persistence.Credential{}, persistence.ErrInvalid
	}
	if input.Provider != "horde" && input.Provider != "cloudflare" && input.Provider != "gemini" {
		return persistence.Credential{}, persistence.ErrInvalid
	}
	secret := input.APIKey
	if input.Provider == "cloudflare" {
		_, accountErr := hex.DecodeString(input.AccountID)
		if !input.WorkersFree || len(input.AccountID) != 32 || accountErr != nil {
			return persistence.Credential{}, persistence.ErrInvalid
		}
		b, _ := json.Marshal(adapters.CloudflareCredential{Token: input.APIKey, AccountID: input.AccountID, FreePlan: true})
		secret = string(b)
	}
	rotate := id != ""
	if !rotate {
		id = uuid.NewString()
	}
	c := persistence.Credential{ID: id, OwnerID: s.OwnerID, Provider: input.Provider, Name: input.Name}
	var err error
	c.Ciphertext, c.Version, err = s.Vault.Seal([]byte(secret), vault.AAD(s.OwnerID, input.Provider, id, "credential"))
	if err != nil {
		return c, err
	}
	if err = s.Store.SaveCredential(ctx, c, rotate); err != nil {
		return c, err
	}
	return s.Store.Credential(ctx, s.OwnerID, id)
}
func (s *Service) Models(ctx context.Context, providerID, credentialID string) ([]provider.Model, error) {
	adapter := s.Adapters[providerID]
	if adapter == nil {
		return nil, persistence.ErrInvalid
	}
	if providerID == "cloudflare" || providerID == "gemini" || credentialID != "" {
		c, err := s.Store.Credential(ctx, s.OwnerID, credentialID)
		if err != nil || c.Provider != providerID || c.Status != "active" {
			return nil, persistence.ErrNotFound
		}
	}
	return adapter.DiscoverModels(ctx, credentialID)
}

type GenerationInput struct {
	Provider   string `json:"provider"`
	Model      string `json:"model_id"`
	Credential string `json:"credential_id,omitempty"`
	Prompt     string `json:"prompt"`
	Aspect     string `json:"aspect_ratio"`
}

func (s *Service) Generate(ctx context.Context, in GenerationInput, idempotency string) (persistence.Generation, error) {
	if in.Provider == "gemini" {
		return persistence.Generation{}, &provider.Error{Category: provider.Billing, Message: "Gemini images require billing and are disabled", AcceptanceKnown: true}
	}
	in.Prompt = strings.TrimSpace(in.Prompt)
	if in.Aspect == "" {
		in.Aspect = "1:1"
	}
	if in.Prompt == "" || len(in.Prompt) > 2000 || in.Aspect != "1:1" && in.Aspect != "16:9" && in.Aspect != "9:16" || len(idempotency) < 1 || len(idempotency) > 200 {
		return persistence.Generation{}, persistence.ErrInvalid
	}
	if in.Provider == "horde" && len(in.Prompt) > 1000 || in.Provider == "cloudflare" && in.Aspect != "1:1" {
		return persistence.Generation{}, persistence.ErrInvalid
	}
	if in.Credential != "" {
		if _, err := uuid.Parse(in.Credential); err != nil {
			return persistence.Generation{}, persistence.ErrInvalid
		}
	}
	bytes, _ := json.Marshal(in)
	hash := sha256.Sum256(bytes)
	if existing, err := s.Store.GenerationForKey(ctx, s.OwnerID, idempotency, hash[:]); err == nil {
		return existing, nil
	} else if !errors.Is(err, persistence.ErrNotFound) {
		return persistence.Generation{}, err
	}
	models, err := s.Models(ctx, in.Provider, in.Credential)
	if err != nil {
		return persistence.Generation{}, err
	}
	found := false
	for _, m := range models {
		if m.ID == in.Model && m.Selectable {
			found = true
		}
	}
	if !found {
		return persistence.Generation{}, &provider.Error{Category: provider.Unsupported, Message: "Model is not currently available", AcceptanceKnown: true}
	}
	id := uuid.NewString()
	encrypted, version, err := s.Vault.Seal([]byte(in.Prompt), vault.AAD(s.OwnerID, in.Provider, id, "prompt"))
	if err != nil {
		return persistence.Generation{}, err
	}
	id, _, err = s.Store.CreateGeneration(ctx, persistence.NewGeneration{ID: id, Owner: s.OwnerID, Provider: in.Provider, Model: in.Model, Credential: in.Credential, Aspect: in.Aspect, Idempotency: idempotency, Hash: hash[:], EncryptedPrompt: encrypted, Version: version})
	if err != nil {
		return persistence.Generation{}, err
	}
	return s.Store.Generation(ctx, s.OwnerID, id)
}
func (s *Service) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := s.Store.Pool.Ping(ctx); err != nil {
		return errors.New("database unavailable")
	}
	return s.Storage.Ready(ctx)
}
