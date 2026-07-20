import { getSession, signOut } from "next-auth/react"

// Base URL for the backend API as reachable from the browser.
export const API_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"

/**
 * authedFetch wraps fetch and attaches the authenticated user's bearer token to
 * every request. If the session is missing or the server rejects the token, the
 * user is sent back to the login page.
 */
export async function authedFetch(
  path: string,
  init: RequestInit = {}
): Promise<Response> {
  const session = await getSession()
  const token = session?.user?.accessToken

  const headers = new Headers(init.headers)
  if (token) {
    headers.set("Authorization", `Bearer ${token}`)
  }
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json")
  }

  const url = path.startsWith("http") ? path : `${API_URL}${path}`
  const res = await fetch(url, { ...init, headers })

  if (res.status === 401) {
    await signOut({ callbackUrl: "/login" })
  }
  return res
}
