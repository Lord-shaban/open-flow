"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import {
  ArrowDownToLine,
  ArrowRight,
  ChevronDown,
  CircleHelp,
  Grid2X2,
  Image as ImageIcon,
  KeyRound,
  LayoutGrid,
  LoaderCircle,
  LogOut,
  Maximize2,
  Search,
  Settings2,
  ShieldCheck,
  Sparkles,
  Trash2,
  Video,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Connections } from "@/components/connections";
import {
  api,
  providerNames,
  type Credential,
  type Model,
  type Generation,
  type Artifact,
} from "@/lib/studio-client";

const pending = new Set(["queued", "submitting", "running"]);
const starters = [
  {
    title: "The last light",
    tag: "CINEMATIC",
    scene: "sunset",
    prompt:
      "A cinematic landscape at golden hour, mountains fading into mist, amber sunlight, film grain, wide composition",
  },
  {
    title: "Somewhere else",
    tag: "IMAGINATION",
    scene: "moon",
    prompt:
      "An otherworldly landscape under a giant moon, quiet purple skies, distant mountains, atmospheric cinematic lighting",
  },
  {
    title: "A quieter world",
    tag: "EXPLORATION",
    scene: "forest",
    prompt:
      "A peaceful misty forest valley at sunrise, deep green hills, soft rays of light, cinematic landscape photography",
  },
];

export function Studio() {
  const [authenticated, setAuthenticated] = useState(false),
    [configured, setConfigured] = useState(false),
    [sessionLoaded, setSessionLoaded] = useState(false);
  const [provider, setProvider] = useState("horde"),
    [credentials, setCredentials] = useState<Credential[]>([]),
    [credential, setCredential] = useState(""),
    [models, setModels] = useState<Model[]>([]),
    [model, setModel] = useState(""),
    [modelLoading, setModelLoading] = useState(false);
  const [generations, setGenerations] = useState<Generation[]>([]),
    [cursor, setCursor] = useState(""),
    [prompt, setPrompt] = useState(""),
    [aspect, setAspect] = useState("1:1"),
    [search, setSearch] = useState(""),
    [filter, setFilter] = useState("all"),
    [compact, setCompact] = useState(false);
  const [settings, setSettings] = useState(false),
    [help, setHelp] = useState(false),
    [access, setAccess] = useState(false),
    [selectedItem, setSelected] = useState<Generation | null>(null),
    [token, setToken] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [historyLoading, setHistoryLoading] = useState(false);
  const textarea = useRef<HTMLTextAreaElement>(null),
    paginated = useRef(false),
    idempotency = useRef<{ payload: string; key: string } | null>(null);
  const selected = selectedItem
    ? (generations.find((g) => g.id === selectedItem.id) ?? selectedItem)
    : null;
  const credentialRevision =
    credentials.find((c) => c.id === credential)?.revision ?? 0;
  const activeCredentials = credentials.filter(
      (c) => c.provider === provider && c.status === "active",
    ),
    activeModel = models.find((m) => m.id === model);
  const refreshHistory = useCallback(async () => {
    const data = await api<{ generations: Generation[]; next_cursor: string }>(
      "generations",
    );
    setGenerations((current) =>
      paginated.current
        ? [
            ...data.generations,
            ...current.filter(
              (g) => !data.generations.some((n) => n.id === g.id),
            ),
          ]
        : data.generations,
    );
    if (!paginated.current) setCursor(data.next_cursor);
  }, []);
  const refreshCredentials = useCallback(async () => {
    setCredentials(await api<Credential[]>("credentials"));
  }, []);
  useEffect(() => {
    fetch("/api/session")
      .then((r) => r.json())
      .then((d) => {
        setAuthenticated(d.authenticated);
        if (d.authenticated) {
          setHistoryLoading(true);
          setModelLoading(true);
        }
        setConfigured(d.configured);
        setSessionLoaded(true);
      })
      .catch(() => setSessionLoaded(true));
  }, []);
  useEffect(() => {
    if (!authenticated) return;
    let canceled = false;
    Promise.all([
      api<{ generations: Generation[]; next_cursor: string }>("generations"),
      api<Credential[]>("credentials"),
    ])
      .then(([history, connections]) => {
        if (canceled) return;
        setGenerations(history.generations);
        setCursor(history.next_cursor);
        setCredentials(connections);
      })
      .catch((e) => {
        if (!canceled) setError(e.message);
      })
      .finally(() => {
        if (!canceled) setHistoryLoading(false);
      });
    return () => {
      canceled = true;
    };
  }, [authenticated]);
  const hasPending = generations.some((g) => pending.has(g.state));
  useEffect(() => {
    if (!authenticated || !hasPending) return;
    const timer = setInterval(() => refreshHistory().catch(() => {}), 4000);
    return () => clearInterval(timer);
  }, [authenticated, hasPending, refreshHistory]);
  useEffect(() => {
    if (!authenticated) return;
    let canceled = false;
    if ((provider === "cloudflare" || provider === "gemini") && !credential) {
      return;
    }
    api<{ models: Model[] }>(
      `models?provider=${provider}&credential_id=${credential}`,
    )
      .then((d) => {
        if (canceled) return;
        setModels(d.models);
        setModel(d.models.find((m) => m.selectable)?.id ?? "");
      })
      .catch((e) => {
        if (!canceled) setError(e.message);
      })
      .finally(() => {
        if (!canceled) setModelLoading(false);
      });
    return () => {
      canceled = true;
    };
  }, [authenticated, provider, credential, credentialRevision]);
  async function unlock(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await fetch("/api/session", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error.message);
      setAuthenticated(true);
      setHistoryLoading(true);
      setModelLoading(true);
      setAccess(false);
      setToken("");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function generate(event?: React.FormEvent) {
    event?.preventDefault();
    if (!authenticated) {
      setAccess(true);
      return;
    }
    if (!prompt.trim() || !model) return;
    setBusy(true);
    setError("");
    setNotice("");
    const payload = JSON.stringify({
      provider,
      model_id: model,
      credential_id: credential,
      prompt: prompt.trim(),
      aspect_ratio: aspect,
    });
    if (idempotency.current?.payload !== payload)
      idempotency.current = { payload, key: crypto.randomUUID() };
    try {
      const result = await api<Generation>("generations", {
        method: "POST",
        headers: { "Idempotency-Key": idempotency.current.key },
        body: payload,
      });
      setGenerations((current) => [
        result,
        ...current.filter((g) => g.id !== result.id),
      ]);
      setPrompt("");
      idempotency.current = null;
      setNotice("Your image is queued. You can leave and come back.");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function remove(g: Generation) {
    setBusy(true);
    setError("");
    try {
      await api(`generations/${g.id}`, { method: "DELETE" });
      setSelected(null);
      setGenerations((current) => current.filter((item) => item.id !== g.id));
      await refreshHistory();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function loadMore() {
    setBusy(true);
    try {
      const data = await api<{
        generations: Generation[];
        next_cursor: string;
      }>(`generations?cursor=${cursor}`);
      paginated.current = true;
      setGenerations((current) => [
        ...current,
        ...data.generations.filter((g) => !current.some((c) => c.id === g.id)),
      ]);
      setCursor(data.next_cursor);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  const visible = generations.filter(
    (g) =>
      (filter === "all" ||
        (filter === "pending" && pending.has(g.state)) ||
        (filter === "failed" && g.state === "reconciliation_required") ||
        g.state === filter) &&
      (g.model_id.toLowerCase().includes(search.toLowerCase()) ||
        g.provider.includes(search.toLowerCase()) ||
        g.id.includes(search.toLowerCase())),
  );
  function chooseProvider(value: string) {
    setModels([]);
    setModel("");
    setModelLoading(
      value !== "cloudflare" ||
        credentials.some((c) => c.provider === value && c.status === "active"),
    );
    setProvider(value);
    setCredential(
      credentials.find((c) => c.provider === value && c.status === "active")
        ?.id ?? "",
    );
    if (value === "cloudflare") setAspect("1:1");
    setError("");
  }
  function openSettings() {
    if (authenticated) setSettings(true);
    else setAccess(true);
  }
  function chooseCredential(id: string) {
    setCredential(id);
    setModels([]);
    setModel("");
    setModelLoading(provider === "horde" || !!id);
    setError("");
  }
  return (
    <div className="studio-shell">
      <header className="studio-header">
        <Link className="wordmark" href="/" aria-label="Open Flow home">
          <span className="flow-symbol">f</span>Open <span>Flow</span>
          <small>LABS</small>
        </Link>
        <div className="header-divider" />
        <span className="project-name">
          My creative space <ChevronDown size={13} />
        </span>
        <div className="header-actions">
          <span className="free-badge">
            <span />
            Free providers
          </span>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Help and setup"
            onClick={() => setHelp(true)}
          >
            <CircleHelp />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Provider settings"
            onClick={openSettings}
          >
            <Settings2 />
          </Button>
          <Button
            className="avatar"
            aria-label={authenticated ? "Sign out" : "Unlock workspace"}
            onClick={() =>
              authenticated
                ? fetch("/api/session", { method: "DELETE" }).then(() => {
                    setAuthenticated(false);
                    setGenerations([]);
                    setCredentials([]);
                    setModels([]);
                    setCredential("");
                    setSelected(null);
                    setSettings(false);
                    setPrompt("");
                    setError("");
                    setNotice("");
                    setCursor("");
                    paginated.current = false;
                    idempotency.current = null;
                  })
                : setAccess(true)
            }
          >
            {authenticated ? <LogOut size={16} /> : "O"}
          </Button>
        </div>
      </header>
      <aside className="studio-sidebar" aria-label="Library navigation">
        <div className="sidebar-title">WORKSPACE</div>
        <Button
          variant="ghost"
          className={filter === "all" ? "nav-item active" : "nav-item"}
          onClick={() => setFilter("all")}
        >
          <LayoutGrid />
          All media <span>{generations.length || ""}</span>
        </Button>
        <Button
          variant="ghost"
          className={filter === "succeeded" ? "nav-item active" : "nav-item"}
          onClick={() => setFilter("succeeded")}
        >
          <ImageIcon />
          Images
        </Button>
        <Button
          variant="ghost"
          className={filter === "pending" ? "nav-item active" : "nav-item"}
          onClick={() => setFilter("pending")}
        >
          <LoaderCircle />
          In progress{" "}
          <span>
            {generations.filter((g) => pending.has(g.state)).length || ""}
          </span>
        </Button>
        <Button
          variant="ghost"
          className={filter === "failed" ? "nav-item active" : "nav-item"}
          onClick={() => setFilter("failed")}
        >
          <X />
          Failed
        </Button>
        <div className="sidebar-rule" />
        <Button variant="ghost" className="nav-item" onClick={openSettings}>
          <KeyRound />
          Connections
        </Button>
        <div className="sidebar-bottom">
          <div className="local-note">
            <ShieldCheck size={16} />
            <div>
              Your keys. Your workspace.
              <small>Private storage · No subscriptions</small>
            </div>
          </div>
          <Button
            variant="ghost"
            className="nav-item"
            onClick={() => setHelp(true)}
          >
            <CircleHelp />
            Getting started
          </Button>
        </div>
      </aside>
      <main id="main" className="studio-main">
        <div className="library-toolbar">
          <div>
            <h1>
              {filter === "all"
                ? "All media"
                : filter === "succeeded"
                  ? "Images"
                  : filter === "pending"
                    ? "In progress"
                    : "Failed generations"}
            </h1>
            <span className="library-count">
              {authenticated
                ? `${generations.length} creations`
                : "A space for your next idea"}
            </span>
          </div>
          <div className="library-tools">
            <div className="search-box">
              <Search size={15} />
              <Input
                aria-label="Search creations"
                placeholder="Search your creations"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
            <Button
              variant="ghost"
              size="icon"
              aria-label={compact ? "Comfortable grid" : "Compact grid"}
              onClick={() => setCompact(!compact)}
            >
              <Grid2X2 />
            </Button>
          </div>
        </div>
        {error || notice ? (
          <div
            className={error ? "status-banner error" : "status-banner"}
            role={error ? "alert" : "status"}
          >
            {error || notice}
            <Button
              variant="ghost"
              size="icon"
              aria-label="Dismiss message"
              onClick={() => {
                setError("");
                setNotice("");
              }}
            >
              <X size={14} />
            </Button>
          </div>
        ) : null}
        {historyLoading ? (
          <div className="loading-state">
            <LoaderCircle className="spin" />
            Opening your library…
          </div>
        ) : generations.length === 0 ? (
          <section className="welcome">
            <div className="welcome-eyebrow">
              <Sparkles size={14} />A LITTLE IDEA. A WHOLE NEW WORLD.
            </div>
            <h2>
              Where will your
              <br />
              <span>imagination take you?</span>
            </h2>
            <p>
              Make something that only you could dream up.
              <br />
              Start with a prompt, and let the story unfold.
            </p>
            <div className="inspiration-grid">
              {starters.map((s, i) => (
                <Button
                  variant="ghost"
                  className={`inspiration-card ${s.scene}`}
                  key={s.title}
                  onClick={() => {
                    setPrompt(s.prompt);
                    textarea.current?.focus();
                  }}
                >
                  <span className="scene-moon" />
                  <span className="scene-mountains" />
                  <span className="scene-mist" />
                  <span className="inspiration-index">0{i + 1}</span>
                  <span className="inspiration-caption">
                    <small>{s.tag}</small>
                    <strong>{s.title}</strong>
                  </span>
                  <span className="inspiration-arrow">
                    <ArrowRight size={17} />
                  </span>
                </Button>
              ))}
            </div>
            <div className="inspiration-hint">
              A few ideas to get you started <span>↗</span>
            </div>
          </section>
        ) : (
          <section
            className={`media-grid ${compact ? "compact" : ""}`}
            aria-label="Generation library"
          >
            {visible.map((g) => (
              <GenerationCard
                key={g.id}
                generation={g}
                onOpen={() => setSelected(g)}
              />
            ))}
            {visible.length === 0 ? (
              <div className="empty-filter">No creations match this view.</div>
            ) : null}
          </section>
        )}
        {cursor ? (
          <Button
            variant="outline"
            className="load-more"
            disabled={busy}
            onClick={loadMore}
          >
            Load more creations
          </Button>
        ) : null}
      </main>
      <div className="composer-dock">
        <form className="composer" onSubmit={generate}>
          <div className="composer-tabs">
            <span className="composer-mode">
              <ImageIcon size={15} />
              Create image
            </span>
            <Button
              type="button"
              variant="ghost"
              className="video-mode"
              onClick={() =>
                setNotice(
                  "Video generation is planned for M2. Images are available now.",
                )
              }
            >
              <Video size={15} />
              Video <small>SOON</small>
            </Button>
            <span className="composer-spacer" />
            <span className="image-count">1 image</span>
          </div>
          <Textarea
            ref={textarea}
            aria-label="Image prompt"
            placeholder="What do you want to create?"
            value={prompt}
            maxLength={provider === "horde" ? 1000 : 2000}
            onChange={(e) => setPrompt(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
                e.preventDefault();
                void generate();
              }
            }}
          />
          <div className="composer-controls">
            <div className="picker">
              <Label className="sr-only" htmlFor="provider">
                Provider
              </Label>
              <select
                id="provider"
                value={provider}
                onChange={(e) => chooseProvider(e.target.value)}
              >
                <option value="horde">AI Horde · Free</option>
                <option value="cloudflare">Cloudflare · Free tier</option>
                <option value="comfyui">ComfyUI · Local</option>
                <option value="training">Training canvas · Not AI</option>
              </select>
              <ChevronDown size={13} />
            </div>
            {activeCredentials.length ? (
              <div className="picker credential-picker">
                <Label className="sr-only" htmlFor="credential">
                  Credential
                </Label>
                <select
                  id="credential"
                  value={credential}
                  onChange={(e) => chooseCredential(e.target.value)}
                >
                  {provider === "horde" ? (
                    <option value="">Anonymous</option>
                  ) : (
                    <option value="">Select key</option>
                  )}
                  {activeCredentials.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
                <ChevronDown size={13} />
              </div>
            ) : null}
            <div className="picker model-picker">
              <Label className="sr-only" htmlFor="model">
                Model
              </Label>
              <select
                id="model"
                value={model}
                onChange={(e) => setModel(e.target.value)}
                disabled={!models.length}
              >
                {!models.length ? (
                  <option>
                    {modelLoading
                      ? "Discovering models…"
                      : authenticated
                        ? "Connect to choose a model"
                        : "Unlock to discover models"}
                  </option>
                ) : null}
                {models.map((m) => (
                  <option key={m.id} value={m.id} disabled={!m.selectable}>
                    {m.display_name}
                  </option>
                ))}
              </select>
              <ChevronDown size={13} />
            </div>
            <div className="picker aspect-picker">
              <Label className="sr-only" htmlFor="aspect">
                Aspect ratio
              </Label>
              <select
                id="aspect"
                value={aspect}
                disabled={provider === "cloudflare"}
                onChange={(e) => setAspect(e.target.value)}
              >
                <option>1:1</option>
                <option>16:9</option>
                <option>9:16</option>
              </select>
              <ChevronDown size={13} />
            </div>
            <Button
              type="submit"
              className="generate-button"
              aria-label={authenticated ? "Generate image" : "Unlock to create"}
              disabled={
                busy ||
                !sessionLoaded ||
                (authenticated && (!prompt.trim() || !activeModel?.selectable))
              }
            >
              {busy ? <LoaderCircle className="spin" /> : <ArrowRight />}
            </Button>
          </div>
          {authenticated && provider === "cloudflare" && !credential ? (
            <Button
              type="button"
              variant="link"
              size="sm"
              onClick={openSettings}
            >
              Connect your Workers Free account
            </Button>
          ) : null}
        </form>
        <div className="composer-footnote">
          {activeModel?.reason ??
            (provider === "horde"
              ? "Free community generation · Anonymous results may be shared · Queue times vary"
              : "Create with your own connection. No paid provider fallback.")}
        </div>
      </div>
      <Dialog
        open={access}
        onOpenChange={(open) => {
          setAccess(open);
          if (!open) setToken("");
        }}
      >
        <DialogContent>
          <div className="dialog-symbol">
            <KeyRound />
          </div>
          <DialogTitle>Make this space yours.</DialogTitle>
          <DialogDescription>
            Unlock your local workspace with the owner token created during
            setup.
          </DialogDescription>
          <form onSubmit={unlock} className="settings-form">
            <Label htmlFor="owner-token">Owner access token</Label>
            <Input
              id="owner-token"
              type="password"
              autoComplete="off"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              required
            />
            <p className="field-hint">
              Stored in an HttpOnly session. Your provider keys stay on the
              server.
            </p>
            {!configured ? (
              <p className="inline-alert">
                The stack is not configured yet. Open Getting started for the
                local setup commands.
              </p>
            ) : null}
            {error ? (
              <p role="alert" className="inline-alert">
                {error}
              </p>
            ) : null}
            <Button type="submit" disabled={busy || !configured}>
              Unlock workspace <ArrowRight size={16} />
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => {
                setAccess(false);
                setHelp(true);
              }}
            >
              Getting started
            </Button>
          </form>
        </DialogContent>
      </Dialog>
      <Connections
        open={settings}
        onOpenChange={setSettings}
        credentials={credentials}
        onRefresh={refreshCredentials}
        onSaved={(c) => {
          if (c.provider !== "gemini") {
            chooseProvider(c.provider);
            setCredential(c.id);
            setModelLoading(true);
          }
        }}
        onRevoked={(id) => {
          if (id === credential) {
            chooseCredential("");
          }
        }}
      />
      <Dialog open={help} onOpenChange={setHelp}>
        <DialogContent>
          <div className="dialog-symbol">
            <Sparkles />
          </div>
          <DialogTitle>A little setup. Endless ideas.</DialogTitle>
          <DialogDescription>
            Open Flow runs locally with free, open-source infrastructure.
          </DialogDescription>
          <div className="help-content">
            <h3>1. Start your local stack</h3>
            <code>
              pnpm setup:local
              <br />
              docker compose --env-file .env -f infra/compose.yaml --profile app
              up -d --build
            </code>
            <p>
              Setup creates your owner token and encryption keys in the ignored
              .env file. Copy OPEN_FLOW_OWNER_TOKEN into Unlock workspace.
            </p>
            <h3>2. Choose a free provider</h3>
            <p>
              <strong>AI Horde</strong> works without an account. Anonymous
              results may be shared; create a free key for more control.
              Community queue times vary.
            </p>
            <p>
              <strong>Cloudflare Workers AI</strong> requires a free account and
              token. Stay on Workers Free without a card; the provider enforces
              its daily allowance.
            </p>
            <p>
              <strong>Training canvas</strong> creates procedural test images
              locally. It is not AI. <strong>ComfyUI</strong> connects to your
              optional local model server.
            </p>
            <h3>3. Create and come back</h3>
            <p>
              Write a prompt, select a discovered model, and generate. Jobs and
              private images survive refreshes. Ctrl/⌘ + Enter submits your
              prompt.
            </p>
            <a
              className="setup-link"
              href="https://github.com/Lord-shaban/open-flow/blob/main/docs/development.md"
              target="_blank"
              rel="noreferrer"
            >
              Read the setup guide ↗
            </a>
          </div>
        </DialogContent>
      </Dialog>
      <Dialog
        open={!!selected}
        onOpenChange={(open) => {
          if (!open) setSelected(null);
        }}
      >
        <DialogContent className="generation-dialog">
          <DialogTitle>
            {selected ? providerNames[selected.provider] : ""} creation
          </DialogTitle>
          <DialogDescription>
            {selected?.state.replaceAll("_", " ")} · {selected?.model_id}
          </DialogDescription>
          {selected?.artifacts[0] ? (
            <PrivateImage artifact={selected.artifacts[0]} download />
          ) : (
            <div className="detail-state">
              {selected?.error_code ?? "Your creation is processing."}
            </div>
          )}
          <p className="field-hint">{selected?.route_reason}</p>
          <div className="generation-meta">{selected?.id}</div>
          {selected && !pending.has(selected.state) ? (
            <Button
              variant="destructive"
              disabled={busy}
              onClick={() => remove(selected)}
            >
              <Trash2 />
              Remove from library
            </Button>
          ) : null}
        </DialogContent>
      </Dialog>
    </div>
  );
}
function GenerationCard({
  generation: g,
  onOpen,
}: {
  generation: Generation;
  onOpen: () => void;
}) {
  return (
    <Button variant="ghost" className="media-card" onClick={onOpen}>
      <div className="media-preview">
        {g.artifacts[0] ? (
          <PrivateImage artifact={g.artifacts[0]} />
        ) : (
          <div className={`generation-placeholder ${g.state}`}>
            <ImageIcon size={30} />
            <span>{g.state.replaceAll("_", " ")}</span>
            {pending.has(g.state) ? (
              <LoaderCircle className="spin" size={17} />
            ) : null}
          </div>
        )}
        <span className="expand-icon">
          <Maximize2 size={16} />
        </span>
      </div>
      <div className="media-caption">
        <strong>{providerNames[g.provider]} creation</strong>
        <span>{g.model_id}</span>
        <small>{g.error_code ?? g.state.replaceAll("_", " ")}</small>
      </div>
    </Button>
  );
}
function PrivateImage({
  artifact,
  download = false,
}: {
  artifact: Artifact;
  download?: boolean;
}) {
  const [url, setURL] = useState(""),
    [failed, setFailed] = useState(false);
  useEffect(() => {
    let alive = true;
    let objectURL = "";
    api<{ url: string }>(`artifacts/${artifact.id}/download`)
      .then(async (d) => {
        const response = await fetch(d.url.replace("/v1/", "/api/flow/"));
        if (!response.ok) throw new Error("Image unavailable");
        objectURL = URL.createObjectURL(await response.blob());
        if (alive) setURL(objectURL);
        else URL.revokeObjectURL(objectURL);
      })
      .catch(() => {
        if (alive) setFailed(true);
      });
    return () => {
      alive = false;
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [artifact.id]);
  if (failed)
    return <span className="image-error">Image expired or unavailable</span>;
  if (!url) return <LoaderCircle className="spin" />;
  return (
    <>
      {/* Private authenticated blob URLs are not compatible with the Next image optimizer. */}
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img className="generated-image" src={url} alt="Your generated image" />
      {download ? (
        <Button asChild className="download-link">
          <a
            href={url}
            download={`open-flow-${artifact.id}.${artifact.content_type === "image/webp" ? "webp" : artifact.content_type === "image/jpeg" ? "jpg" : "png"}`}
          >
            <ArrowDownToLine />
            Download image
          </a>
        </Button>
      ) : null}
    </>
  );
}
