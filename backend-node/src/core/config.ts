/**
 * Reads the process environment into a validated Settings object.
 *
 * Variable names and fallbacks are identical to the other three backends', so
 * one deployment environment can drive any of them.
 */

export const DATABASE_URL_VARIABLE = 'DATABASE_URL'
export const JWT_SECRET_VARIABLE = 'JWT_SECRET'
export const PORT_VARIABLE = 'PORT'
export const ALLOWED_ORIGINS_VARIABLE = 'ALLOWED_ORIGINS'

/**
 * Must stay byte-identical to JwtOptions.LocalDevelopmentSecret in the .NET
 * API, or locally issued tokens stop crossing backends.
 */
export const LOCAL_DEVELOPMENT_SECRET =
  'finance-tracker-local-development-signing-key-please-override'

export const DEFAULT_ALLOWED_ORIGINS = 'http://localhost:5173'

/** Keeps this API off the .NET (5000), Go (8081) and Python (8082) ports. */
export const DEFAULT_PORT = 8083

export class ConfigError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ConfigError'
  }
}

export interface Settings {
  readonly databaseUrl: string
  readonly jwtSecret: string
  readonly port: number
  readonly allowedOrigins: readonly string[]
}

/** Splits the comma-separated origin list, falling back to the default. */
export function parseOrigins(raw: string | undefined): string[] {
  const value = raw && raw.trim() ? raw : DEFAULT_ALLOWED_ORIGINS
  const origins = value
    .split(',')
    .map((origin) => origin.trim())
    .filter((origin) => origin.length > 0)
  return origins.length > 0 ? origins : [DEFAULT_ALLOWED_ORIGINS]
}

/** Reads the environment. DATABASE_URL is required; the rest have defaults. */
export function loadSettings(env: NodeJS.ProcessEnv = process.env): Settings {
  const databaseUrl = (env[DATABASE_URL_VARIABLE] ?? '').trim()
  if (!databaseUrl) {
    throw new ConfigError(
      `config: ${DATABASE_URL_VARIABLE} is required (postgres:// connection string)`,
    )
  }

  const jwtSecret = (env[JWT_SECRET_VARIABLE] ?? '').trim() || LOCAL_DEVELOPMENT_SECRET

  let port = DEFAULT_PORT
  const rawPort = (env[PORT_VARIABLE] ?? '').trim()
  if (rawPort) {
    port = Number(rawPort)
    if (!Number.isInteger(port) || port <= 0 || port > 65535) {
      throw new ConfigError(
        `config: ${PORT_VARIABLE} must be a TCP port number, got ${rawPort}`,
      )
    }
  }

  return {
    databaseUrl,
    jwtSecret,
    port,
    allowedOrigins: parseOrigins(env[ALLOWED_ORIGINS_VARIABLE]),
  }
}
