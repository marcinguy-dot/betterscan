import NextAuth from "next-auth"
import Google from "next-auth/providers/google"
import GitHub from "next-auth/providers/github"
import Credentials from "next-auth/providers/credentials"

const INTERNAL_API_URL =
  process.env.INTERNAL_API_URL ||
  process.env.NEXT_PUBLIC_API_URL ||
  "http://localhost:8080"

const gluuConfigured =
  process.env.GLUU_CLIENT_ID &&
  process.env.GLUU_CLIENT_SECRET &&
  process.env.GLUU_ISSUER

const gluuProvider = {
  id: "gluu",
  name: "Gluu",
  type: "oidc" as const,
  issuer: process.env.GLUU_ISSUER,
  clientId: process.env.GLUU_CLIENT_ID,
  clientSecret: process.env.GLUU_CLIENT_SECRET,
  authorization: { params: { scope: "openid email profile" } },
}

export const { handlers, auth, signIn, signOut } = NextAuth({
  session: { strategy: "jwt" },
  providers: [
    Google({
      clientId: process.env.GOOGLE_CLIENT_ID || "",
      clientSecret: process.env.GOOGLE_CLIENT_SECRET || "",
    }),
    GitHub({
      clientId: process.env.GITHUB_CLIENT_ID || "",
      clientSecret: process.env.GITHUB_CLIENT_SECRET || "",
    }),
    Credentials({
      name: "Email and Password",
      credentials: {
        email: { label: "Email", type: "email" },
        password: { label: "Password", type: "password" },
      },
      async authorize(credentials) {
        if (!credentials?.email || !credentials?.password) {
          return null
        }
        const res = await fetch(`${INTERNAL_API_URL}/api/v1/auth/login`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            email: credentials.email,
            password: credentials.password,
          }),
        })
        if (!res.ok) {
          return null
        }
        const data = await res.json()
        if (!data?.token || !data?.user) {
          return null
        }
        return {
          id: data.user.id,
          email: data.user.email,
          name: data.user.name,
          role: data.user.role,
          accessToken: data.token,
        }
      },
    }),
    ...(gluuConfigured ? [gluuProvider] : []),
  ],
  callbacks: {
    async jwt({ token, account, user }: any) {
      if (user) {
        token.id = user.id as string
        token.role = (user as { role?: string }).role
        if ((user as { accessToken?: string }).accessToken) {
          token.accessToken = (user as { accessToken?: string }).accessToken
        }
      }
      if (account && account.access_token) {
        token.accessToken = account.access_token
      }
      return token
    },
    async session({ session, token }: any) {
      if (session.user) {
        session.user.id = token.id as string
        session.user.accessToken = token.accessToken as string
        session.user.role = token.role as string | undefined
      }
      return session
    },
  },
  pages: {
    signIn: "/login",
  },
})
