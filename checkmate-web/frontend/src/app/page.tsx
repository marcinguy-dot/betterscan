"use client"

import { useEffect, useState } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart"
import { Bar, BarChart, Line, LineChart, XAxis, YAxis } from "recharts"

interface DashboardStats {
  total_projects: number
  total_scans: number
  running_scans: number
  total_findings: number
  critical_count: number
  high_count: number
  medium_count: number
  low_count: number
}

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

interface TrendData {
  date: string
  severity: string
  count: number
}

export default function Dashboard() {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [scans, setScans] = useState<Scan[]>([])
  const [trends, setTrends] = useState<TrendData[]>([])
  const [loading, setLoading] = useState(true)

  const fetchData = async () => {
    try {
      const [statsRes, scansRes, trendsRes] = await Promise.all([
        fetch("http://localhost:8080/api/v1/dashboard/stats"),
        fetch("http://localhost:8080/api/v1/scans?limit=10"),
        fetch("http://localhost:8080/api/v1/dashboard/trends"),
      ])

      const [statsData, scansData, trendsData] = await Promise.all([
        statsRes.json(),
        scansRes.json(),
        trendsRes.json(),
      ])

      setStats(statsData)
      setScans(scansData)
      setTrends(trendsData)
    } catch (error) {
      console.error("Failed to fetch data:", error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    const loadData = async () => {
      await fetchData()
    }
    loadData()
  }, [])

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

  // Prepare trend data for charts
  const severityTrends = trends.reduce((acc, trend) => {
    if (!acc[trend.date]) {
      acc[trend.date] = { date: trend.date, critical: 0, high: 0, medium: 0, low: 0 }
    }
    const severityKey = trend.severity as 'critical' | 'high' | 'medium' | 'low'
    if (severityKey in acc[trend.date]) {
      acc[trend.date][severityKey] = trend.count
    }
    return acc
  }, {} as Record<string, { date: string; critical: number; high: number; medium: number; low: number }>)

  const chartData = Object.values(severityTrends).slice(-30) // Last 30 days

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-lg">Loading dashboard...</div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-7xl mx-auto">
        <div className="flex justify-between items-center mb-8">
          <h1 className="text-3xl font-bold">Security Dashboard</h1>
          <Button onClick={fetchData}>Refresh</Button>
        </div>

        {/* Stats Cards */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-gray-600">Total Projects</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold">{stats?.total_projects || 0}</div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-gray-600">Total Scans</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold">{stats?.total_scans || 0}</div>
              <div className="text-sm text-gray-500">{stats?.running_scans || 0} running</div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-gray-600">Total Findings</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold">{stats?.total_findings || 0}</div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-gray-600">Critical Issues</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-red-600">{stats?.critical_count || 0}</div>
            </CardContent>
          </Card>
        </div>

        {/* Severity Breakdown */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
          <Card className="border-red-200">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-red-600">Critical</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-red-600">{stats?.critical_count || 0}</div>
            </CardContent>
          </Card>

          <Card className="border-orange-200">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-orange-600">High</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-orange-600">{stats?.high_count || 0}</div>
            </CardContent>
          </Card>

          <Card className="border-yellow-200">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-yellow-600">Medium</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-yellow-600">{stats?.medium_count || 0}</div>
            </CardContent>
          </Card>

          <Card className="border-blue-200">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-blue-600">Low</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-blue-600">{stats?.low_count || 0}</div>
            </CardContent>
          </Card>
        </div>

        <Tabs defaultValue="scans" className="space-y-6">
          <TabsList>
            <TabsTrigger value="scans">Recent Scans</TabsTrigger>
            <TabsTrigger value="trends">Vulnerability Trends</TabsTrigger>
          </TabsList>

          <TabsContent value="scans">
            <Card>
              <CardHeader>
                <CardTitle>Recent Scans</CardTitle>
                <CardDescription>Latest security scan results</CardDescription>
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
                    <div className="text-center py-8 text-gray-500">No scans yet</div>
                  )}
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="trends">
            <Card>
              <CardHeader>
                <CardTitle>Vulnerability Trends</CardTitle>
                <CardDescription>Vulnerability counts over time</CardDescription>
              </CardHeader>
              <CardContent>
                <ChartContainer config={{}}>
                  <LineChart data={chartData}>
                    <XAxis dataKey="date" />
                    <YAxis />
                    <ChartTooltip content={<ChartTooltipContent />} />
                    <Line type="monotone" dataKey="critical" stroke="#ef4444" strokeWidth={2} name="Critical" />
                    <Line type="monotone" dataKey="high" stroke="#f97316" strokeWidth={2} name="High" />
                    <Line type="monotone" dataKey="medium" stroke="#eab308" strokeWidth={2} name="Medium" />
                    <Line type="monotone" dataKey="low" stroke="#3b82f6" strokeWidth={2} name="Low" />
                  </LineChart>
                </ChartContainer>
              </CardContent>
            </Card>

            <Card className="mt-6">
              <CardHeader>
                <CardTitle>Severity Distribution</CardTitle>
                <CardDescription>Current vulnerability distribution</CardDescription>
              </CardHeader>
              <CardContent>
                <ChartContainer config={{}}>
                  <BarChart data={[
                    { name: "Critical", value: stats?.critical_count || 0, fill: "#ef4444" },
                    { name: "High", value: stats?.high_count || 0, fill: "#f97316" },
                    { name: "Medium", value: stats?.medium_count || 0, fill: "#eab308" },
                    { name: "Low", value: stats?.low_count || 0, fill: "#3b82f6" },
                  ]}>
                    <XAxis dataKey="name" />
                    <YAxis />
                    <ChartTooltip content={<ChartTooltipContent />} />
                    <Bar dataKey="value" />
                  </BarChart>
                </ChartContainer>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  )
}
