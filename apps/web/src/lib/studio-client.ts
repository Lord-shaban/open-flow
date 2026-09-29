export type Model = {
  id: string;
  provider_id: string;
  display_name: string;
  selectable: boolean;
  reason: string;
};
export type Credential = {
  id: string;
  name: string;
  provider: string;
  status: string;
  revision: number;
};
export type Artifact = { id: string; content_type: string; bytes: number };
export type Generation = {
  id: string;
  state: string;
  provider: string;
  model_id: string;
  aspect_ratio: string;
  created_at: string;
  artifacts: Artifact[];
  route_reason: string;
  error_code?: string;
};
export const providerNames: Record<string, string> = {
  horde: "AI Horde",
  cloudflare: "Cloudflare",
  comfyui: "ComfyUI",
  training: "Training canvas",
  gemini: "Gemini",
};
export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api/flow/${path}`, {
    ...init,
    cache: "no-store",
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  const data = await response.json();
  if (!response.ok) throw new Error(data.error?.message ?? "Request failed");
  return data;
}
