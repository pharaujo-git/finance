/** The identity layer, which has to interoperate with the other three APIs. */

import { pbkdf2Sync } from 'node:crypto'
import jwt from 'jsonwebtoken'
import { describe, expect, it } from 'vitest'
import { LOCAL_DEVELOPMENT_SECRET } from '../src/core/config.js'
import {
  DEFAULT_ITERATIONS,
  DEFAULT_PRF,
  InvalidTokenError,
  TokenService,
  hashPassword,
  verifyPassword,
} from '../src/core/security.js'

const SECRET = LOCAL_DEVELOPMENT_SECRET
const USER_ID = '3d4d6087-3456-450e-8392-3887ad469b95'

/** Builds an Identity v3 blob with chosen parameters. */
function blob(
  password: string,
  { prf, iterations, saltLen, subkeyLen }: {
    prf: number
    iterations: number
    saltLen: number
    subkeyLen: number
  },
): string {
  const digests: Record<number, string> = { 0: 'sha1', 1: 'sha256', 2: 'sha512' }
  const salt = Buffer.alloc(saltLen, 1)
  const subkey = pbkdf2Sync(password, salt, iterations, subkeyLen, digests[prf]!)

  const header = Buffer.alloc(13)
  header.writeUInt8(0x01, 0)
  header.writeUInt32BE(prf, 1)
  header.writeUInt32BE(iterations, 5)
  header.writeUInt32BE(saltLen, 9)
  return Buffer.concat([header, salt, subkey]).toString('base64')
}

describe('password hashing', () => {
  it('round-trips', () => {
    expect(verifyPassword(hashPassword('Passw0rd!123'), 'Passw0rd!123')).toBe('success')
  })

  it('rejects a wrong password', () => {
    expect(verifyPassword(hashPassword('Passw0rd!123'), 'Passw0rd!124')).toBe('failed')
  })

  it('writes the current defaults', () => {
    const raw = Buffer.from(hashPassword('x'), 'base64')
    expect(raw.readUInt8(0)).toBe(0x01)
    expect(raw.readUInt32BE(1)).toBe(DEFAULT_PRF)
    expect(raw.readUInt32BE(5)).toBe(DEFAULT_ITERATIONS)
    expect(raw.readUInt32BE(9)).toBe(16)
    expect(raw.length).toBe(13 + 16 + 32)
  })

  it('uses a fresh salt each time', () => {
    const first = hashPassword('same')
    const second = hashPassword('same')
    expect(first).not.toBe(second)
  })

  it('accepts a weaker blob but asks for a rehash', () => {
    // A blob the .NET API would have written years ago.
    const stored = blob('legacy', { prf: 1, iterations: 10_000, saltLen: 16, subkeyLen: 32 })
    expect(verifyPassword(stored, 'legacy')).toBe('successRehashNeeded')
  })

  it('flags a short subkey for rehash', () => {
    const stored = blob('legacy', {
      prf: 2,
      iterations: DEFAULT_ITERATIONS,
      saltLen: 16,
      subkeyLen: 16,
    })
    expect(verifyPassword(stored, 'legacy')).toBe('successRehashNeeded')
  })

  it.each([
    ['not base64 at all!!', 'junk'],
    ['', 'empty'],
    [Buffer.concat([Buffer.from([0x02]), Buffer.alloc(40)]).toString('base64'), 'wrong marker'],
    [Buffer.from([0x01]).toString('base64'), 'truncated'],
  ])('rejects a malformed blob (%s)', (stored) => {
    expect(verifyPassword(stored, 'anything')).toBe('failed')
  })

  it('rejects a salt below 128 bits', () => {
    const stored = blob('x', { prf: 2, iterations: 1000, saltLen: 8, subkeyLen: 32 })
    expect(verifyPassword(stored, 'x')).toBe('failed')
  })

  it('rejects an absurd iteration count', () => {
    const raw = Buffer.from(hashPassword('x'), 'base64')
    raw.writeUInt32BE(20_000_000, 5)
    expect(verifyPassword(raw.toString('base64'), 'x')).toBe('failed')
  })
})

describe('TokenService', () => {
  const service = new TokenService(SECRET)

  it('round-trips', () => {
    const principal = service.validate(service.issue(USER_ID, 'owner@example.com'))
    expect(principal.userId).toBe(USER_ID)
    expect(principal.email).toBe('owner@example.com')
  })

  it('writes the shared claim set', () => {
    const claims = jwt.verify(service.issue(USER_ID, 'owner@example.com'), SECRET, {
      audience: 'finance-tracker',
    }) as jwt.JwtPayload

    expect(claims.iss).toBe('finance-tracker')
    expect(claims.aud).toBe('finance-tracker')
    expect(claims.sub).toBe(USER_ID)
    expect(claims.email).toBe('owner@example.com')
    expect(claims.exp! - claims.iat!).toBe(7 * 24 * 3600)
    expect(claims.nbf).toBe(claims.iat)
    expect(typeof claims.jti).toBe('string')
  })

  it('rejects another secret', () => {
    const issued = service.issue(USER_ID, 'a@b.c')
    expect(() => new TokenService('a different signing key').validate(issued)).toThrow(
      InvalidTokenError,
    )
  })

  it('rejects a tampered payload', () => {
    const [header, payload, signature] = service.issue(USER_ID, 'a@b.c').split('.')
    const forged = `${header}.${payload!.slice(0, -4)}AAAA.${signature}`
    expect(() => service.validate(forged)).toThrow(InvalidTokenError)
  })

  it('rejects an expired token', () => {
    const longAgo = new Date(Date.now() - 8 * 24 * 3600 * 1000)
    expect(() => service.validate(service.issue(USER_ID, 'a@b.c', longAgo))).toThrow(
      InvalidTokenError,
    )
  })

  it('rejects the none algorithm', () => {
    const forged = jwt.sign({ sub: USER_ID, iss: 'finance-tracker', aud: 'finance-tracker' }, '', {
      algorithm: 'none',
    })
    expect(() => service.validate(forged)).toThrow(InvalidTokenError)
  })

  it('rejects a foreign issuer', () => {
    const forged = jwt.sign(
      { sub: USER_ID, iss: 'somewhere-else', aud: 'finance-tracker' },
      SECRET,
      { expiresIn: '1d' },
    )
    expect(() => service.validate(forged)).toThrow(InvalidTokenError)
  })

  it('rejects a non-uuid subject', () => {
    const forged = jwt.sign(
      { sub: 'not-a-uuid', iss: 'finance-tracker', aud: 'finance-tracker' },
      SECRET,
      { expiresIn: '1d' },
    )
    expect(() => service.validate(forged)).toThrow(InvalidTokenError)
  })
})
