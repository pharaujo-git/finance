/**
 * The two things all four backends must agree on byte for byte: the ASP.NET
 * Core Identity v3 password blob and the HS256 bearer token.
 *
 * A token minted here must be accepted by the .NET, Go and Python APIs and
 * vice versa, so none of the constants below are free to change.
 */

import { pbkdf2Sync, randomBytes, randomUUID, timingSafeEqual } from 'node:crypto'
import jwt from 'jsonwebtoken'

// --- password hashing -------------------------------------------------------
//
// Layout of the base64-encoded version-3 blob:
//
//   byte  0      format marker, always 0x01
//   bytes 1..4   PRF id, uint32 big-endian
//   bytes 5..8   iteration count, uint32 big-endian
//   bytes 9..12  salt length in bytes, uint32 big-endian
//   bytes 13..   salt, then the derived subkey (rest of the blob)

const FORMAT_MARKER_V3 = 0x01
const HEADER_LEN = 13

/** PRF 0 and 1 are only ever read from stored blobs written before the defaults moved. */
const PRF_DIGESTS: Record<number, string> = { 0: 'sha1', 1: 'sha256', 2: 'sha512' }

export const DEFAULT_PRF = 2
export const DEFAULT_ITERATIONS = 100_000
export const DEFAULT_SALT_LEN = 16
export const DEFAULT_SUBKEY_LEN = 32

/** Identity rejects salts under 128 bits, and treats absurd counts as corruption. */
const MIN_SALT_LEN = 16
const MAX_ITERATIONS = 10_000_000

export type PasswordOutcome = 'failed' | 'success' | 'successRehashNeeded'

/** Derives a new Identity v3 blob using the current defaults. */
export function hashPassword(password: string): string {
  const salt = randomBytes(DEFAULT_SALT_LEN)
  const subkey = pbkdf2Sync(
    password,
    salt,
    DEFAULT_ITERATIONS,
    DEFAULT_SUBKEY_LEN,
    PRF_DIGESTS[DEFAULT_PRF]!,
  )

  const header = Buffer.alloc(HEADER_LEN)
  header.writeUInt8(FORMAT_MARKER_V3, 0)
  header.writeUInt32BE(DEFAULT_PRF, 1)
  header.writeUInt32BE(DEFAULT_ITERATIONS, 5)
  header.writeUInt32BE(DEFAULT_SALT_LEN, 9)

  return Buffer.concat([header, salt, subkey]).toString('base64')
}

/**
 * Checks a password against a stored blob. A correct password whose blob uses
 * parameters weaker than the current defaults reports successRehashNeeded,
 * exactly as Identity's own hasher does, so the caller can upgrade it.
 */
export function verifyPassword(storedHash: string, password: string): PasswordOutcome {
  let blob: Buffer
  try {
    blob = Buffer.from(storedHash, 'base64')
    // Buffer.from ignores junk rather than throwing, so check the round trip.
    if (blob.toString('base64').replace(/=+$/, '') !== storedHash.replace(/=+$/, '')) {
      return 'failed'
    }
  } catch {
    return 'failed'
  }

  if (blob.length < HEADER_LEN + 1 || blob.readUInt8(0) !== FORMAT_MARKER_V3) return 'failed'

  const prfId = blob.readUInt32BE(1)
  const iterations = blob.readUInt32BE(5)
  const saltLen = blob.readUInt32BE(9)

  // Both bounds also keep the slicing below in range.
  if (saltLen < MIN_SALT_LEN || HEADER_LEN + saltLen >= blob.length) return 'failed'
  if (iterations === 0 || iterations > MAX_ITERATIONS) return 'failed'

  const digest = PRF_DIGESTS[prfId]
  if (digest === undefined) return 'failed'

  const saltEnd = HEADER_LEN + saltLen
  const salt = blob.subarray(HEADER_LEN, saltEnd)
  const expected = blob.subarray(saltEnd)
  if (expected.length < 16) return 'failed'

  const actual = pbkdf2Sync(password, salt, iterations, expected.length, digest)
  if (!timingSafeEqual(actual, expected)) return 'failed'

  if (
    prfId !== DEFAULT_PRF ||
    iterations < DEFAULT_ITERATIONS ||
    expected.length < DEFAULT_SUBKEY_LEN
  ) {
    return 'successRehashNeeded'
  }
  return 'success'
}

// --- bearer tokens ----------------------------------------------------------

/** Issuer, audience and the 7-day lifetime match JwtOptions in the .NET API. */
export const ISSUER = 'finance-tracker'
export const AUDIENCE = 'finance-tracker'
const TOKEN_LIFETIME_SECONDS = 7 * 24 * 60 * 60
/** Matches the JwtBearer ClockSkew of one minute. */
const CLOCK_LEEWAY_SECONDS = 60
const SIGNING_ALG = 'HS256'

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

/** Raised for every rejection reason, so callers cannot leak which check failed. */
export class InvalidTokenError extends Error {
  constructor(reason: string) {
    super(reason)
    this.name = 'InvalidTokenError'
  }
}

export interface Principal {
  userId: string
  email: string
}

export class TokenService {
  private readonly secret: string

  constructor(secret: string) {
    this.secret = secret
  }

  /** Mints a token. The claim set matches what the other three backends write. */
  issue(userId: string, email: string, now: Date = new Date()): string {
    const issuedAt = Math.floor(now.getTime() / 1000)
    return jwt.sign(
      {
        iss: ISSUER,
        aud: AUDIENCE,
        exp: issuedAt + TOKEN_LIFETIME_SECONDS,
        iat: issuedAt,
        nbf: issuedAt,
        sub: userId,
        email,
        jti: randomUUID(),
      },
      this.secret,
      { algorithm: SIGNING_ALG },
    )
  }

  /** Checks signature, issuer, audience and lifetime, then extracts the caller. */
  validate(token: string): Principal {
    let claims: jwt.JwtPayload
    try {
      const decoded = jwt.verify(token, this.secret, {
        algorithms: [SIGNING_ALG],
        issuer: ISSUER,
        audience: AUDIENCE,
        clockTolerance: CLOCK_LEEWAY_SECONDS,
      })
      if (typeof decoded === 'string') throw new InvalidTokenError('payload is not an object')
      claims = decoded
    } catch (cause) {
      throw new InvalidTokenError(cause instanceof Error ? cause.message : 'invalid token')
    }

    if (typeof claims.exp !== 'number') throw new InvalidTokenError('missing expiry')

    const subject = typeof claims.sub === 'string' ? claims.sub : ''
    if (!UUID_PATTERN.test(subject)) throw new InvalidTokenError('subject is not a uuid')

    return { userId: subject, email: typeof claims.email === 'string' ? claims.email : '' }
  }
}
