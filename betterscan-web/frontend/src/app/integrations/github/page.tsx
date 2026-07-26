"use client"

import { Suspense, useEffect, useState } from "react"
import { useSession } from "next-auth/react"
import { useRouter, useSearchParams } from "next/navigation"
import { authedFetch } from "@/lib/api"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"

interface VcsConnection {
  id: string
  provider: string
  auth_type: string
  host: string
  display_name: string
  external_id: string
  created_at: string
}

interface ProvidersStatus {
  github_app: { configured: boolean; mock: boolean; slug?: string }
  generic_pat: { configured: boolean }
}

function IntegrationsContent() {
  const { status } = useSession()
  const router = useRouter()
  const searchParams = useSearchParams()
  const [providers, setProviders] = useState<ProvidersStatus | null>(null)
  const [connections, setConnections] = useState<VcsConnection[]>([])
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(true)
  const [patForm, setPatForm] = useState({
    provider: "gitlab",
    host: "",
    display_name: "",
    token: "",
  })

  const load = async () => {
    setLoading(true)
    setError("")
    try {
      const [pRes, cRes] = await Promise.all([
        authedFetch("/api/v1/vcs/providers"),
        authedFetch("/api/v1/vcs/connections"),
      ])
      if (pRes.ok) setProviders(await pRes.json())
      if (cRes.ok) setConnections(await cRes.json())
    } catch {
      setError("Failed to load integrations")
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (status === "unauthenticated") {
      router.push("/login")
      return
    }
    if (status === "authenticated") {
      load()
      const installationId = searchParams.get("installation_id")
      if (installationId) {
        authedFetch("/api/v1/vcs/github/finalize", {
          method: "POST",
          body: JSON.stringify({ installation_id: installationId }),
        }).then(() => load())
      }
    }
  }, [status, router, searchParams])

  const installGitHub = async () => {
    setError("")
    const res = await authedFetch("/api/v1/vcs/github/install-url")
    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      setError(data.error || "Could not start GitHub App install")
      return
    }
    if (data.mock) {
      await load()
      return
    }
    if (data.url) window.location.href = data.url
  }

  const disconnect = async (id: string) => {
    await authedFetch(`/api/v1/vcs/connections/${id}`, { method: "DELETE" })
    await load()
  }

  const addPat = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")
    const res = await authedFetch("/api/v1/vcs/connections", {
      method: "POST",
      body: JSON.stringify({
        provider: patForm.provider,
        host: patForm.host || undefined,
        display_name: patForm.display_name,
        token: patForm.token,
      }),
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      setError(data.error || "Failed to save connection")
      return
    }
    setPatForm({ provider: "gitlab", host: "", display_name: "", token: "" })
    await load()
  }

  if (status === "loading" || loading) {
    return <div className="p-8 text-gray-500">Loading integrations…</div>
  }

  return (
    <div className="max-w-4xl mx-auto p-6 space-y-8" data-testid="integrations-page">
      <div>
        <h1 className="text-2xl font-bold">Integrations</h1>
        <p className="text-sm text-gray-600 mt-1">
          Connect GitHub App for install-based access, or add a PAT for GitLab, Bitbucket, or any git host.
          Private clones use short-lived or stored credentials at scan time.
        </p>
      </div>

      {error && (
        <div className="rounded-md bg-red-50 p-3 text-sm text-red-700" data-testid="integrations-error">
          {error}
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle>GitHub App</CardTitle>
          <CardDescription>
            Modern install flow with repository permissions (Contents: Read).{" "}
            {providers?.github_app?.mock && (
              <Badge variant="secondary" className="ml-1">
                mock mode
              </Badge>
            )}
            {!providers?.github_app?.configured && (
              <span className="block mt-1 text-amber-700">
                Not configured — set GITHUB_APP_* or GITHUB_APP_MOCK=1 on the backend.
              </span>
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button
            onClick={installGitHub}
            disabled={!providers?.github_app?.configured}
            data-testid="github-install-btn"
          >
            {providers?.github_app?.mock ? "Connect mock GitHub App" : "Install GitHub App"}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Token / PAT (GitLab, Bitbucket, GitHub, other)</CardTitle>
          <CardDescription>
            Same worker path for all hosts: HTTPS clone as oauth2 / x-token-auth / x-access-token / git.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={addPat} className="grid gap-4 sm:grid-cols-2" data-testid="pat-form">
            <div className="space-y-2">
              <Label htmlFor="provider">Provider</Label>
              <select
                id="provider"
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm"
                value={patForm.provider}
                onChange={(e) => setPatForm({ ...patForm, provider: e.target.value })}
                data-testid="pat-provider"
              >
                <option value="gitlab">GitLab</option>
                <option value="bitbucket">Bitbucket</option>
                <option value="github">GitHub (PAT)</option>
                <option value="generic">Generic git</option>
              </select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="host">Host (optional for SaaS defaults)</Label>
              <Input
                id="host"
                placeholder="gitlab.com / bitbucket.org / git.example.com"
                value={patForm.host}
                onChange={(e) => setPatForm({ ...patForm, host: e.target.value })}
                data-testid="pat-host"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="display_name">Display name</Label>
              <Input
                id="display_name"
                value={patForm.display_name}
                onChange={(e) => setPatForm({ ...patForm, display_name: e.target.value })}
                data-testid="pat-name"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="token">Token / PAT</Label>
              <Input
                id="token"
                type="password"
                required
                value={patForm.token}
                onChange={(e) => setPatForm({ ...patForm, token: e.target.value })}
                data-testid="pat-token"
              />
            </div>
            <div className="sm:col-span-2">
              <Button type="submit" data-testid="pat-submit">
                Save connection
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Connected accounts</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3" data-testid="connections-list">
          {connections.length === 0 && (
            <p className="text-sm text-gray-500">No connections yet.</p>
          )}
          {connections.map((c) => (
            <div
              key={c.id}
              className="flex items-center justify-between border rounded-md p-3"
              data-testid={`connection-${c.id}`}
            >
              <div>
                <div className="font-medium">{c.display_name}</div>
                <div className="text-xs text-gray-500">
                  {c.provider} · {c.auth_type} · {c.host}
                </div>
              </div>
              <Button variant="outline" size="sm" onClick={() => disconnect(c.id)}>
                Disconnect
              </Button>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  )
}

export default function IntegrationsPage() {
  return (
    <Suspense fallback={<div className="p-8 text-gray-500">Loading integrations…</div>}>
      <IntegrationsContent />
    </Suspense>
  )
}
