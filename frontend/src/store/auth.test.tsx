import { describe, it, expect } from 'vitest'
import { parseToken } from './auth'

function makeJwt(payload: Record<string, unknown>): string {
  const header = btoa('{"alg":"HS256","typ":"JWT"}')
  const body = btoa(JSON.stringify(payload))
  return `${header}.${body}.fakesig`
}

const futureExp = () => Math.floor(Date.now() / 1000) + 3600
const pastExp = () => Math.floor(Date.now() / 1000) - 60

describe('parseToken', () => {
  it('returns a user for a valid unexpired token', () => {
    const u = parseToken(makeJwt({
      user_id: 7,
      username: 'alice',
      role: 'admin',
      exp: futureExp(),
    }))
    expect(u).toEqual({
      id: 7,
      username: 'alice',
      role: 'admin',
      passwordChangeRequired: false,
    })
  })

  it('surfaces the pwd_change_required flag', () => {
    const u = parseToken(makeJwt({
      user_id: 1,
      username: 'admin',
      role: 'admin',
      pwd_change_required: true,
      exp: futureExp(),
    }))
    expect(u?.passwordChangeRequired).toBe(true)
  })

  // The flash-of-authenticated-UI fix: an expired token must look like no
  // token at all, so the app shell never renders for that one frame before
  // the first 401 kicks the user to /login.
  it('returns null for an expired token', () => {
    expect(parseToken(makeJwt({
      user_id: 1, username: 'admin', role: 'admin', exp: pastExp(),
    }))).toBeNull()
  })

  it('returns null when exp is missing', () => {
    expect(parseToken(makeJwt({
      user_id: 1, username: 'admin', role: 'admin',
    }))).toBeNull()
  })

  it('returns null when exp is non-numeric', () => {
    expect(parseToken(makeJwt({
      user_id: 1, username: 'admin', role: 'admin', exp: 'soon',
    }))).toBeNull()
  })

  it('returns null for a malformed token', () => {
    expect(parseToken('not.a.jwt')).toBeNull()
    expect(parseToken('')).toBeNull()
    expect(parseToken('only-one-segment')).toBeNull()
  })
})
