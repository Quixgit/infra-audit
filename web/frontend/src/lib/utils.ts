import type React from 'react'
import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatDuration(startedAt: string, finishedAt?: string | null): string {
  const start = new Date(startedAt).getTime()
  const end = finishedAt ? new Date(finishedAt).getTime() : Date.now()
  const ms = end - start
  const secs = Math.floor(ms / 1000)
  if (secs < 60) return `${secs}s`
  const mins = Math.floor(secs / 60)
  if (mins < 60) return `${mins}m ${secs % 60}s`
  return `${Math.floor(mins / 60)}h ${mins % 60}m`
}

export function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

export function formatDateShort(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString()
}

/** Copies text to clipboard. Returns true on success.
 *  Uses the modern Clipboard API where available (requires HTTPS / secure context),
 *  and falls back to the legacy execCommand approach for plain-HTTP environments.
 *
 *  For the best HTTP-fallback, prefer passing the optional `inputRef` so the
 *  function can select text directly on an already-visible <input> element —
 *  this is far more reliable than a hidden textarea on modern browsers. */
export async function copyToClipboard(
  text: string,
  inputRef?: React.RefObject<HTMLInputElement>,
): Promise<boolean> {
  // Modern Clipboard API — works only in secure contexts (HTTPS / localhost)
  if (typeof navigator !== 'undefined' && navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // permission denied — fall through to legacy
    }
  }

  // Preferred fallback: select text in the supplied visible input element
  if (inputRef?.current) {
    const el = inputRef.current
    el.focus()
    el.select()
    el.setSelectionRange(0, el.value.length)
    try {
      const ok = document.execCommand('copy')
      return ok
    } catch {
      return false
    }
  }

  // Last-resort fallback: inject a tiny textarea into the viewport and copy from it
  try {
    const el = document.createElement('textarea')
    el.value = text
    // Must stay inside the viewport — off-screen elements are blocked by modern browsers
    el.style.cssText =
      'position:fixed;top:0;left:0;width:2em;height:2em;padding:0;border:none;' +
      'outline:none;box-shadow:none;background:transparent;opacity:0;z-index:9999;'
    document.body.appendChild(el)
    el.focus()
    el.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(el)
    return ok
  } catch {
    return false
  }
}
