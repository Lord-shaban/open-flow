"use client";
import { useState } from "react";
import { Check, LoaderCircle, Plus, ShieldCheck, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api, providerNames, type Credential } from "@/lib/studio-client";
export function Connections({
  open,
  onOpenChange,
  credentials,
  onRefresh,
  onSaved,
  onRevoked,
}: {
  open: boolean;
  onOpenChange: (value: boolean) => void;
  credentials: Credential[];
  onRefresh: () => Promise<void>;
  onSaved: (c: Credential) => void;
  onRevoked: (id: string) => void;
}) {
  const [provider, setProvider] = useState("cloudflare"),
    [name, setName] = useState(""),
    [key, setKey] = useState(""),
    [account, setAccount] = useState(""),
    [free, setFree] = useState(false),
    [rotating, setRotating] = useState(""),
    [message, setMessage] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const c = await api<Credential>(
        rotating ? `credentials/${rotating}` : "credentials",
        {
          method: rotating ? "PUT" : "POST",
          body: JSON.stringify({
            name,
            provider,
            api_key: key,
            account_id: account,
            workers_free_confirmed: free,
          }),
        },
      );
      setKey("");
      setRotating("");
      await onRefresh();
      onSaved(c);
      setMessage(
        "Key encrypted and saved. Test the connection before creating.",
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function test(id: string) {
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const r = await api<{ latency_ms: number }>(`credentials/${id}/test`, {
        method: "POST",
        body: "{}",
      });
      setMessage(
        `Connection verified · ${r.latency_ms} ms · No image was generated.`,
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function revoke(id: string) {
    setBusy(true);
    setError("");
    try {
      await api(`credentials/${id}`, { method: "DELETE" });
      onRevoked(id);
      await onRefresh();
      setMessage("Credential revoked.");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        onOpenChange(value);
        if (!value) {
          setKey("");
          setRotating("");
          setMessage("");
          setError("");
        }
      }}
    >
      <DialogContent className="settings-dialog">
        <div className="dialog-symbol">
          <ShieldCheck />
        </div>
        <DialogTitle>Your connections</DialogTitle>
        <DialogDescription>
          Bring a free-tier key. Secrets are encrypted, never returned to your
          browser.
        </DialogDescription>
        <div className="connection-list">
          {credentials.map((c) => (
            <div className="connection" key={c.id}>
              <div>
                <strong>{c.name}</strong>
                <small>
                  {providerNames[c.provider]} · {c.status} · v{c.revision}
                </small>
              </div>
              {c.status === "active" ? (
                <div>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={busy}
                    onClick={() => test(c.id)}
                  >
                    Test
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={busy}
                    onClick={() => {
                      setRotating(c.id);
                      setProvider(c.provider);
                      setName(c.name);
                      setKey("");
                    }}
                  >
                    Rotate
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={`Revoke ${c.name}`}
                    disabled={busy}
                    onClick={() => revoke(c.id)}
                  >
                    <Trash2 size={14} />
                  </Button>
                </div>
              ) : (
                <Check size={16} />
              )}
            </div>
          ))}
        </div>
        <form className="settings-form" onSubmit={save}>
          <div className="form-row">
            <div>
              <Label htmlFor="key-provider">Provider</Label>
              <select
                id="key-provider"
                className="form-select"
                value={provider}
                disabled={!!rotating}
                onChange={(e) => setProvider(e.target.value)}
              >
                <option value="cloudflare">Cloudflare Workers AI</option>
                <option value="horde">AI Horde</option>
                <option value="gemini">Gemini · Discovery only</option>
              </select>
            </div>
            <div>
              <Label htmlFor="key-name">Connection name</Label>
              <Input
                id="key-name"
                value={name}
                maxLength={120}
                placeholder="My free account"
                onChange={(e) => setName(e.target.value)}
                required
              />
            </div>
          </div>
          <Label htmlFor="api-key">
            {provider === "cloudflare" ? "Workers AI API token" : "API key"}
          </Label>
          <Input
            id="api-key"
            type="password"
            autoComplete="off"
            value={key}
            onChange={(e) => setKey(e.target.value)}
            minLength={10}
            maxLength={4096}
            required
          />
          {provider === "cloudflare" ? (
            <>
              <Label htmlFor="account-id">Cloudflare account ID</Label>
              <Input
                id="account-id"
                value={account}
                onChange={(e) => setAccount(e.target.value)}
                pattern="[a-fA-F0-9]{32}"
                required
              />
              <Label className="checkbox-label">
                <input
                  type="checkbox"
                  checked={free}
                  onChange={(e) => setFree(e.target.checked)}
                  required
                />
                This is a Workers Free account without a payment method.
              </Label>
              <a
                className="setup-link"
                href="https://developers.cloudflare.com/workers-ai/get-started/rest-api/"
                target="_blank"
                rel="noreferrer"
              >
                Get a free Workers AI token ↗
              </a>
            </>
          ) : (
            <a
              className="setup-link"
              href={
                provider === "horde"
                  ? "https://aihorde.net/register"
                  : "https://aistudio.google.com/apikey"
              }
              target="_blank"
              rel="noreferrer"
            >
              Official key setup ↗
            </a>
          )}
          {provider === "gemini" ? (
            <p className="inline-alert">
              Gemini image models require paid billing. Only connection testing
              and model discovery are enabled.
            </p>
          ) : null}
          {error ? (
            <p className="inline-alert" role="alert">
              {error}
            </p>
          ) : null}
          {message ? (
            <p className="connection-success" role="status">
              {message}
            </p>
          ) : null}
          <Button type="submit" disabled={busy}>
            {rotating ? "Rotate encrypted key" : "Save encrypted connection"}
            {busy ? <LoaderCircle className="spin" /> : <Plus size={16} />}
          </Button>
          {rotating ? (
            <Button
              type="button"
              variant="ghost"
              onClick={() => setRotating("")}
            >
              Cancel rotation
            </Button>
          ) : null}
        </form>
      </DialogContent>
    </Dialog>
  );
}
