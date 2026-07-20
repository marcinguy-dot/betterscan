"use client"

import { useEffect, useState } from "react"
import { useSession } from "next-auth/react"
import { useRouter } from "next/navigation"
import { authedFetch } from "@/lib/api"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"

interface Finding {
  id: string
  scan_id: string
  severity: string
  message: string
  file: string
  line: number
  code: string
  analyzer: string
  confidence: string
  fingerprint: string
  is_false_positive: boolean
  false_positive_reason: string
  created_at: string
}

export default function FindingsPage() {
  const router = useRouter()
  const { status } = useSession()
  const [findings, setFindings] = useState<Finding[]>([])
  const [loading, setLoading] = useState(true)

  const fetchFindings = async () => {
    try {
      const res = await authedFetch("/api/v1/findings")
      const data = await res.json()
      setFindings(Array.isArray(data) ? data : [])
    } catch (error) {
      console.error("Failed to fetch findings:", error)
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

    fetchFindings()
  }, [status])

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case "critical":
        return "bg-red-500"
      case "high":
        return "bg-orange-500"
      case "medium":
        return "bg-yellow-500"
      case "low":
        return "bg-blue-500"
      default:
        return "bg-gray-500"
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-lg">Loading findings...</div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-7xl mx-auto">
        <div className="flex justify-between items-center mb-8">
          <h1 className="text-3xl font-bold">Findings</h1>
          <Button onClick={fetchFindings}>Refresh</Button>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>All Findings</CardTitle>
            <CardDescription>Security vulnerabilities discovered across all scans</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {findings.map((finding) => (
                <div key={finding.id} className="p-4 border rounded-lg">
                  <div className="flex items-center gap-2 mb-1">
                    <Badge className={getSeverityColor(finding.severity)}>{finding.severity}</Badge>
                    {finding.is_false_positive && (
                      <Badge variant="outline" className="border-gray-400 text-gray-500">False Positive</Badge>
                    )}
                    <span className="text-sm text-gray-500">{new Date(finding.created_at).toLocaleString()}</span>
                  </div>
                  <h3 className="font-semibold mt-1">{finding.message || finding.code}</h3>
                  <div className="flex gap-4 mt-2 text-xs text-gray-500">
                    {finding.file && (
                      <span>
                        <span className="font-medium">File:</span> {finding.file}
                        {finding.line > 0 && `:${finding.line}`}
                      </span>
                    )}
                    {finding.code && (
                      <span>
                        <span className="font-medium">Code:</span> {finding.code}
                      </span>
                    )}
                    {finding.analyzer && (
                      <span>
                        <span className="font-medium">Analyzer:</span> {finding.analyzer}
                      </span>
                    )}
                  </div>
                </div>
              ))}
              {findings.length === 0 && (
                <div className="text-center py-8 text-gray-500">No findings found</div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}