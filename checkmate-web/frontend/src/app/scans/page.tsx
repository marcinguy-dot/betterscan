"use client"

import { useEffect, useState } from "react"
import { useSession } from "next-auth/react"
import { useRouter } from "next/navigation"
import { authedFetch } from "@/lib/api"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"

interface Scan {
  id: string
  project_id: string
  status: string
  strategy: string
  tools: string
  duration_ms: number
  total_issues: number
  critical_count: number
  high_count: number
  medium_count: number
  low_count: number
  started_at: string
  completed_at: string
  created_at: string
}

export default function ScansPage() {
  const router = useRouter()
  const { status } = useSession()
  const [scans, setScans] = useState<Scan[]>([])
  const [loading, setLoading] = useState(true)

  const fetchScans = async () => {
    try {
      const res = await authedFetch("/api/v1/scans?limit=50")
      const data = await res.json()
      setScans(Array.isArray(data) ? data : [])
    } catch (error) {
      console.error("Failed to fetch scans:", error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (status === "unauthenticated") {
      router.push("/login")
      return
    }
    if (status !== "authenticated") return

    fetchScans()
  }, [status])

  const getStatusColor = (status: string) => {
    switch (status) {
      case "completed":
        return "bg-green-500"
      case "running":
        return "bg-blue-500"
      case "failed":
        return "bg-red-500"
      default:
        return "bg-gray-500"
    }
  }

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
    return `${(ms / 60000).toFixed(1)}m`
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-lg">Loading scans...</div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-7xl mx-auto">
        <div className="flex justify-between items-center mb-8">
          <h1 className="text-3xl font-bold">Scans</h1>
          <Button onClick={fetchScans}>Refresh</Button>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>All Scans</CardTitle>
            <CardDescription>Security scan results across all projects</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {scans.map((scan) => (
                <div key={scan.id} className="flex items-center justify-between p-4 border rounded-lg">
                  <div className="flex-1">
                    <div className="flex items-center gap-2 mb-1">
                      <Badge className={getStatusColor(scan.status)}>{scan.status}</Badge>
                      <span className="text-sm text-gray-500">{new Date(scan.created_at).toLocaleString()}</span>
                    </div>
                    <div className="text-sm">
                      <span className="font-medium">Tools:</span> {scan.tools}
                      <span className="ml-4 font-medium">Strategy:</span> {scan.strategy}
                    </div>
                    <div className="flex gap-4 mt-2 text-sm">
                      <span className="text-red-600">C: {scan.critical_count}</span>
                      <span className="text-orange-600">H: {scan.high_count}</span>
                      <span className="text-yellow-600">M: {scan.medium_count}</span>
                      <span className="text-blue-600">L: {scan.low_count}</span>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-lg font-bold">{scan.total_issues} issues</div>
                    <div className="text-sm text-gray-500">{formatDuration(scan.duration_ms)}</div>
                  </div>
                </div>
              ))}
              {scans.length === 0 && (
                <div className="text-center py-8 text-gray-500">No scans found</div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}