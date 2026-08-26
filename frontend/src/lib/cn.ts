type ClassValue = string | false | null | undefined

/** Join class names, dropping falsy entries. */
export function cn(...values: ClassValue[]): string {
  return values.filter(Boolean).join(' ')
}
