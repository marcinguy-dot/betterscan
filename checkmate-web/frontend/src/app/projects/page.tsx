"use client"

import { useEffect, useState } from "react"
import { useSession } from "next-auth/react"
import { authedFetch } from "@/lib/api"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"

interface Project {
  id: string
  name: string
  description: string
  repo_url: string
  repo_branch: string
  language: string
  created_at: string
}

export default function ProjectsPage() {
  const { status } = useSession()
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(true)
  const [showDialog, setShowDialog] = useState(false)
  const [newProject, setNewProject] = useState({
    name: "",
    description: "",
    repo_url: "",
    repo_branch: "main",
    language: "",
  })
  const [formError, setFormError] = useState("")
  const [scanningProjects, setScanningProjects] = useState<Set<string>>(new Set())

  const triggerScan = async (projectId: string) => {
    setScanningProjects((prev) => new Set(prev).add(projectId))
    try {
      await authedFetch("/api/v1/scans", {
        method: "POST",
        body: JSON.stringify({
          project_id: projectId,
          strategy: "parallel",
          tools: "opengrep,trivy",
        }),
      })
    } catch (error) {
      console.error("Failed to trigger scan:", error)
    } finally {
      setScanningProjects((prev) => {
        const next = new Set(prev)
        next.delete(projectId)
        return next
      })
    }
  }

  // Mirror the backend's allowlist so obviously invalid input is caught early.
  // The backend remains the authoritative validation boundary.
  const validateNewProject = (): string | null => {
    const name = newProject.name.trim()
    if (!name) return "Project name is required"
    if (name.length > 200) return "Project name is too long"

    const repoUrl = newProject.repo_url.trim()
    if (!repoUrl) return "Repository URL is required"
    const scpLike = /^[A-Za-z0-9._-]+@[A-Za-z0-9._-]+:.+$/.test(repoUrl)
    if (!scpLike) {
      try {
        const parsed = new URL(repoUrl)
        const allowed = ["http:", "https:", "ssh:", "git:"]
        if (!allowed.includes(parsed.protocol)) return "Repository URL must use http(s), ssh or git"
      } catch {
        return "Repository URL is not a valid URL"
      }
    }

    const branch = newProject.repo_branch.trim()
    if (branch && !/^[A-Za-z0-9._/-]+$/.test(branch)) return "Branch name contains invalid characters"

    return null
  }

  const fetchProjects = async () => {
    try {
      const res = await authedFetch("/api/v1/projects")
      const data = await res.json()
      setProjects(data)
    } catch (error) {
      console.error("Failed to fetch projects:", error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    const loadProjects = async () => {
      if (status === "authenticated") {
        await fetchProjects()
      } else if (status === "unauthenticated") {
        setLoading(false)
      }
    }
    loadProjects()
  }, [status])

  const handleCreateProject = async () => {
    const validationError = validateNewProject()
    if (validationError) {
      setFormError(validationError)
      return
    }
    setFormError("")
    try {
      const res = await authedFetch("/api/v1/projects", {
        method: "POST",
        body: JSON.stringify(newProject),
      })
      if (res.ok) {
        const createdProject = await res.json()
        setShowDialog(false)
        setNewProject({ name: "", description: "", repo_url: "", repo_branch: "main", language: "" })
        await fetchProjects()
        // Auto-trigger a scan for the newly created project
        await triggerScan(createdProject.id)
      } else {
        const data = await res.json().catch(() => ({}))
        setFormError(data.error || "Failed to create project")
      }
    } catch (error) {
      console.error("Failed to create project:", error)
      setFormError("Failed to create project")
    }
  }

  if (status === "loading" || loading) {
    return <div className="flex items-center justify-center min-h-screen">Loading...</div>
  }

  if (status === "unauthenticated") {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <Button onClick={() => (window.location.href = "/login")}>Sign In</Button>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-7xl mx-auto">
        <div className="flex justify-between items-center mb-8">
          <h1 className="text-3xl font-bold">Projects</h1>
          <Button onClick={() => setShowDialog(true)}>Add Project</Button>
          <Dialog open={showDialog} onOpenChange={setShowDialog}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create New Project</DialogTitle>
                <DialogDescription>Add a new code repository to scan</DialogDescription>
              </DialogHeader>
              <div className="space-y-4">
                <div>
                  <Label htmlFor="name">Project Name</Label>
                  <Input
                    id="name"
                    value={newProject.name}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setNewProject({ ...newProject, name: e.target.value })}
                  />
                </div>
                <div>
                  <Label htmlFor="description">Description</Label>
                  <Textarea
                    id="description"
                    value={newProject.description}
                    onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setNewProject({ ...newProject, description: e.target.value })}
                  />
                </div>
                <div>
                  <Label htmlFor="repo_url">Repository URL</Label>
                  <Input
                    id="repo_url"
                    value={newProject.repo_url}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setNewProject({ ...newProject, repo_url: e.target.value })}
                    placeholder="https://github.com/user/repo"
                  />
                </div>
                <div>
                  <Label htmlFor="repo_branch">Branch</Label>
                  <Input
                    id="repo_branch"
                    value={newProject.repo_branch}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setNewProject({ ...newProject, repo_branch: e.target.value })}
                  />
                </div>
                <div>
                  <Label htmlFor="language">Language</Label>
                  <Input
                    id="language"
                    value={newProject.language}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setNewProject({ ...newProject, language: e.target.value })}
                    placeholder="go, python, java, etc."
                  />
                </div>
                {formError && (
                  <p className="text-sm text-red-600" role="alert">{formError}</p>
                )}
                <Button onClick={handleCreateProject} className="w-full">
                  Create Project
                </Button>
              </div>
            </DialogContent>
          </Dialog>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {projects.map((project) => (
            <Card key={project.id}>
              <CardHeader>
                <CardTitle>{project.name}</CardTitle>
                <CardDescription>{project.description}</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Badge variant="outline">{project.language}</Badge>
                    <Badge variant="outline">{project.repo_branch}</Badge>
                  </div>
                  <div className="text-sm text-gray-500 truncate">{project.repo_url}</div>
                  <div className="text-xs text-gray-400">
                    Added {new Date(project.created_at).toLocaleDateString()}
                  </div>
                  <div className="flex gap-2 mt-4">
                    <Button variant="outline" className="flex-1">
                      View Scans
                    </Button>
                    <Button
                      variant="default"
                      className="flex-1"
                      disabled={scanningProjects.has(project.id)}
                      onClick={() => triggerScan(project.id)}
                    >
                      {scanningProjects.has(project.id) ? "Scanning..." : "Scan Now"}
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
          {projects.length === 0 && (
            <div className="col-span-full text-center py-12 text-gray-500">
              No projects yet. Add your first project to get started.
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
