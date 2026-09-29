import {
  ArrowDown,
  ArrowRight,
  BookOpen,
  Boxes,
  CircleDot,
  Code2,
  Database,
  GitBranch,
  Layers3,
  Radio,
  ShieldCheck,
  Workflow,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import Link from "next/link";

const repo = "https://github.com/Lord-shaban/open-flow";
const phases = [
  {
    id: "M0",
    name: "Project foundation",
    detail: "Repository, contracts, workflows & engineering standards",
    state: "Current phase",
  },
  {
    id: "M1",
    name: "Gemini images",
    detail: "Secure credentials, durable image jobs & private artifacts",
    state: "Planned",
  },
  {
    id: "M2",
    name: "Video & resilience",
    detail: "Veo operations, recovery, history & usage events",
    state: "Planned",
  },
  {
    id: "M3",
    name: "Provider routing",
    detail: "Vertex AI, community adapters & explainable decisions",
    state: "Planned",
  },
  {
    id: "M4",
    name: "Production hardening",
    detail: "Authorization, telemetry, deployment & recovery labs",
    state: "Planned",
  },
];
const modules = [
  {
    icon: Boxes,
    name: "Providers & models",
    detail: "Your credentials. A common adapter contract.",
    phase: "M1 / M3",
  },
  {
    icon: Workflow,
    name: "Durable generations",
    detail: "Temporal workflows for images and long-running video.",
    phase: "M1 / M2",
  },
  {
    icon: GitBranch,
    name: "Explainable routing",
    detail: "Gemini first, with observable selection and failures.",
    phase: "M3",
  },
  {
    icon: Radio,
    name: "Usage & analytics",
    detail: "Kafka events with durable, idempotent consumers.",
    phase: "M2 / M4",
  },
];

export default function Home() {
  return (
    <div className="shell">
      <aside className="sidebar">
        <Link className="brand" href="/" aria-label="Open Flow home">
          <span className="brand-mark">
            <Layers3 size={22} />
          </span>
          open flow<span className="brand-dot">.</span>
        </Link>
        <div className="workspace-label">DEVELOPMENT WORKSPACE</div>
        <nav aria-label="Workspace navigation">
          <a className="nav-link active" href="#main">
            <CircleDot size={17} /> Overview
          </a>
          <a className="nav-link" href="#workspace">
            <Boxes size={17} /> Product modules
          </a>
          <a className="nav-link" href="#architecture">
            <Workflow size={17} /> Architecture
          </a>
          <a className="nav-link" href="#roadmap">
            <GitBranch size={17} /> Roadmap
          </a>
          <a className="nav-link" href={repo + "/blob/main/docs/api.md"}>
            <Code2 size={17} /> Developer / API
          </a>
        </nav>
        <div className="sidebar-note">
          <ShieldCheck size={19} />
          <strong>Bring your own keys</strong>
          <p>
            Credentials stay on your server. Provider connections arrive in M1.
          </p>
        </div>
        <a className="nav-link sidebar-bottom" href={repo}>
          <Code2 size={17} /> Open-source on GitHub <ArrowRight size={14} />
        </a>
      </aside>
      <div className="content">
        <header className="topbar">
          <span>
            Workspace <span className="slash">/</span> Overview
          </span>
          <a href={repo + "/blob/main/docs/development.md"}>
            <BookOpen size={15} /> Documentation <ArrowRight size={14} />
          </a>
        </header>
        <main id="main">
          <div className="eyebrow">
            <span className="status-dot" /> FOUNDATION RELEASE{" "}
            <span className="version">v0.1.0-dev</span>
          </div>
          <div className="hero">
            <div>
              <h1>
                Your media pipeline,
                <br />
                <span>under control.</span>
              </h1>
              <p>
                A Gemini-first gateway for AI images and video.
                <br className="desktop-break" /> Open architecture. Durable
                workflows. Your infrastructure.
              </p>
            </div>
            <span className="hero-symbol" aria-hidden="true">
              <Layers3 size={96} strokeWidth={1} />
            </span>
          </div>
          <div className="actions">
            <Button asChild>
              <a href={repo + "/issues"}>
                Explore the project <ArrowRight size={16} />
              </a>
            </Button>
            <Button asChild variant="outline">
              <a href="#architecture">
                See the architecture <ArrowDown size={16} />
              </a>
            </Button>
          </div>
          <div className="notice">
            <CircleDot size={18} />
            <p>
              <strong>The foundation is ready to explore.</strong> Media
              generation and provider connections are being built through the
              roadmap below. This workspace makes no provider calls.
            </p>
          </div>

          <section id="workspace" aria-labelledby="workspace-title">
            <div className="section-heading">
              <div>
                <span className="section-kicker">THE PLATFORM</span>
                <h2 id="workspace-title">One workspace. Every stage.</h2>
              </div>
              <span className="quiet-badge">Planned capabilities</span>
            </div>
            <div className="module-grid">
              {modules.map(({ icon: Icon, name, detail, phase }) => (
                <article className="module" key={name}>
                  <span className="module-icon">
                    <Icon size={22} />
                  </span>
                  <span className="module-phase">{phase}</span>
                  <h3>{name}</h3>
                  <p>{detail}</p>
                </article>
              ))}
            </div>
          </section>

          <section id="architecture" aria-labelledby="architecture-title">
            <div className="section-heading">
              <div>
                <span className="section-kicker">BUILT TO UNDERSTAND</span>
                <h2 id="architecture-title">A system you can reason about.</h2>
              </div>
              <a
                className="text-link"
                href={repo + "/blob/main/docs/architecture.md"}
              >
                Read the design <ArrowRight size={15} />
              </a>
            </div>
            <div className="architecture">
              <div className="flow-line">
                <div>
                  <Code2 size={20} />
                  <strong>Go API</strong>
                  <span>Accept & authorize</span>
                </div>
                <ArrowRight className="flow-arrow" size={18} />
                <div>
                  <Workflow size={20} />
                  <strong>Temporal</strong>
                  <span>Orchestrate & recover</span>
                </div>
                <ArrowRight className="flow-arrow" size={18} />
                <div>
                  <Boxes size={20} />
                  <strong>Provider adapter</strong>
                  <span>Gemini first</span>
                </div>
              </div>
              <div className="event-lane">
                <Database size={16} />
                <span>PostgreSQL + transactional outbox</span>
                <ArrowRight size={15} />
                <Radio size={16} />
                <strong>Kafka</strong>
                <span className="event-suffix">
                  Lifecycle events → usage & analytics
                </span>
              </div>
              <p className="architecture-caption">
                Target generation flow. Foundation probes are available;
                generation dispatch and outbox delivery are tracked in M1.
              </p>
            </div>
          </section>

          <section id="roadmap" aria-labelledby="roadmap-title">
            <div className="section-heading">
              <div>
                <span className="section-kicker">BUILDING IN THE OPEN</span>
                <h2 id="roadmap-title">From foundation to production.</h2>
              </div>
              <a className="text-link" href={repo + "/milestones"}>
                All milestones <ArrowRight size={15} />
              </a>
            </div>
            <div className="roadmap">
              {phases.map((phase, index) => (
                <a
                  href={repo + "/milestone/" + (index + 1)}
                  className="roadmap-row"
                  key={phase.id}
                >
                  <span
                    className={"phase-id " + (index === 0 ? "current" : "")}
                  >
                    {phase.id}
                  </span>
                  <div>
                    <h3>{phase.name}</h3>
                    <p>{phase.detail}</p>
                  </div>
                  <span
                    className={"phase-state " + (index === 0 ? "current" : "")}
                  >
                    {phase.state}
                  </span>
                  <ArrowRight size={16} />
                </a>
              ))}
            </div>
          </section>
          <footer>
            <span>Open Flow · Made to build, inspect, and extend.</span>
            <a href={repo + "/blob/main/LICENSE"}>
              MIT licensed <ArrowRight size={13} />
            </a>
          </footer>
        </main>
      </div>
    </div>
  );
}
