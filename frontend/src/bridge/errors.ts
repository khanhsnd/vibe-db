export type ErrorCode =
  | 'INVALID_PROFILE'
  | 'INVALID_NAME'
  | 'PROFILE_NOT_FOUND'
  | 'SESSION_NOT_FOUND'
  | 'BAD_CREDENTIALS'
  | 'UNKNOWN_DATABASE'
  | 'UNKNOWN_SCHEMA'
  | 'UNKNOWN_TABLE'
  | 'TABLE_NOT_EDITABLE'
  | 'EMPTY_STATEMENT'
  | 'SQL_ERROR'
  | 'FILE_UNREADABLE'
  | 'UNSUPPORTED'
  | 'QUERY_CANCELLED'
  | 'HOST_UNREACHABLE'
  | 'TIMEOUT'
  | 'TLS_UNSUPPORTED'
  | 'TLS_NOT_VERIFIED'
  | 'PASSWORD_NOT_FOUND'
  | 'INTERNAL'

export class AppError extends Error {
  readonly code: ErrorCode
  readonly field: string | undefined
  readonly detail: string | undefined

  constructor(code: ErrorCode, message: string, field?: string, detail?: string) {
    super(message)
    this.name = 'AppError'
    this.code = code
    this.field = field
    this.detail = detail
  }
}

function isErrorCode(value: unknown): value is ErrorCode {
  return typeof value === 'string'
}

export function toAppError(thrown: unknown): AppError {
  // `call` already converted it. Parsing an AppError's message again would lose the
  // code and report every failure as INTERNAL.
  if (thrown instanceof AppError) return thrown

  const raw =
    thrown instanceof Error ? thrown.message : typeof thrown === 'string' ? thrown : String(thrown)

  try {
    const parsed: unknown = JSON.parse(raw)
    if (parsed && typeof parsed === 'object' && 'code' in parsed && 'message' in parsed) {
      const { code, message, field, detail } = parsed as Record<string, unknown>
      if (isErrorCode(code) && typeof message === 'string') {
        return new AppError(
          code,
          message,
          typeof field === 'string' ? field : undefined,
          typeof detail === 'string' ? detail : undefined,
        )
      }
    }
  } catch {
    // Not our envelope; fall through to the generic case below.
  }

  return new AppError('INTERNAL', 'Something went wrong.', undefined, raw)
}

export async function call<T>(operation: () => Promise<T>): Promise<T> {
  try {
    return await operation()
  } catch (thrown) {
    throw toAppError(thrown)
  }
}
