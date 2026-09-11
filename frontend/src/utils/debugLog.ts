// Export diagnostic categories instead of raw server errors, which can contain
// signed URLs, credentials and full local paths.
export function errorCategory(message: string): string {
  if (/cancel|aborted/i.test(message)) return 'cancelled'
  if (/timeout|deadline/i.test(message)) return 'timeout'
  if (/401|403|unauthori|forbidden|credential|authentication/i.test(message)) return 'authentication-or-permission'
  if (/404|not found/i.test(message)) return 'not-found'
  if (/429|rate.limit/i.test(message)) return 'rate-limited'
  if (/network|connection|dial tcp|dns|tls|socket|EOF/i.test(message)) return 'network'
  if (/unsupported|invalid.*(file|format)|metadata/i.test(message)) return 'file-or-metadata'
  if (/\b5\d\d\b/.test(message)) return 'server-error'
  return 'other-error'
}

export function safeName(value: string): string {
  return (value.split(/[\\/]/).pop() || '')
    .replace(/AF1Qip[\w.-]*/g, '[album key]')
    .replace(/[\w.+-]+@[\w.-]+\.[a-z]{2,}/gi, '[email]')
}
