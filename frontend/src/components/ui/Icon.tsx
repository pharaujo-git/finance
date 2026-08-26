import { cn } from '@/lib/cn'

/** 24x24 stroke icons. Keeping them inline avoids an icon-font dependency. */
export const ICON_PATHS = {
  dashboard: 'M4 4h7v7H4zM13 4h7v4h-7zM13 10h7v10h-7zM4 13h7v7H4z',
  transactions: 'M4 8h13m0 0-3-3m3 3-3 3M20 16H7m0 0 3-3m-3 3 3 3',
  accounts: 'M3 8a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2m-16 0v9a2 2 0 0 0 2 2h13a2 2 0 0 0 2-2v-3m-3-3h4v6h-4a3 3 0 0 1 0-6Z',
  budgets: 'M12 3a9 9 0 1 0 9 9h-9V3Z M14 3.5A9 9 0 0 1 20.5 10H14V3.5Z',
  goals: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Zm0-4.5a4.5 4.5 0 1 0 0-9 4.5 4.5 0 0 0 0 9Zm0-3a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3Z',
  reports: 'M4 20V10m5 10V4m5 16v-7m5 7V8',
  settings:
    'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z M19.4 15a1.6 1.6 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.6 1.6 0 0 0-1.8-.3 1.6 1.6 0 0 0-1 1.5v.2a2 2 0 1 1-4 0v-.1a1.6 1.6 0 0 0-1-1.5 1.6 1.6 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.6 1.6 0 0 0 .3-1.8 1.6 1.6 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.6 1.6 0 0 0 1.5-1 1.6 1.6 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.6 1.6 0 0 0 1.8.3H9a1.6 1.6 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.6 1.6 0 0 0 1 1.5 1.6 1.6 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.6 1.6 0 0 0-.3 1.8V9a1.6 1.6 0 0 0 1.5 1h.2a2 2 0 1 1 0 4h-.1a1.6 1.6 0 0 0-1.5 1Z',
  close: 'M6 6l12 12M18 6L6 18',
  plus: 'M12 5v14M5 12h14',
  minus: 'M5 12h14',
  edit: 'M4 20h4l10.5-10.5a2.1 2.1 0 0 0-3-3L5 17v3Z',
  trash: 'M4 7h16M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2m-9 0 1 13a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1l1-13',
  sun: 'M12 17a5 5 0 1 0 0-10 5 5 0 0 0 0 10ZM12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4',
  moon: 'M20 14.5A8.5 8.5 0 0 1 9.5 4a8.5 8.5 0 1 0 10.5 10.5Z',
  menu: 'M4 6h16M4 12h16M4 18h16',
  logout: 'M15 17l5-5-5-5m5 5H9M12 21H6a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h6',
  chevronLeft: 'M15 6l-6 6 6 6',
  chevronRight: 'M9 6l6 6-6 6',
  chevronDown: 'M6 9l6 6 6-6',
  upload: 'M12 16V4m0 0L8 8m4-4 4 4M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2',
  download: 'M12 4v12m0 0 4-4m-4 4-4-4M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2',
  search: 'M11 18a7 7 0 1 0 0-14 7 7 0 0 0 0 14Zm5.5-1.5L21 21',
  check: 'M5 13l4 4L19 7',
  archive: 'M3 7h18v3H3zM5 10v9a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-9M9 14h6',
  checking: 'M3 10h18M3 7a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z',
  savings: 'M4 12a7 7 0 0 1 7-7h3a7 7 0 0 1 7 7v3a2 2 0 0 1-2 2h-1v2h-3v-2H9v2H6v-2H5a1 1 0 0 1-1-1zM7 11h.01',
  creditCard: 'M3 9h18M3 7a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2zM7 15h4',
  cash: 'M2 7h20v10H2zM12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z',
  investment: 'M4 18 10 12l3 3 7-7m0 0h-5m5 0v5',
  repeat: 'M4 9V7a2 2 0 0 1 2-2h11m0 0-3-3m3 3-3 3M20 15v2a2 2 0 0 1-2 2H7m0 0 3 3m-3-3 3-3',
  filter: 'M4 5h16l-6 7v6l-4 2v-8L4 5Z',
  wallet:
    'M3 8a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2m-16 0v9a2 2 0 0 0 2 2h13a2 2 0 0 0 2-2v-3m-3-3h4v6h-4a3 3 0 0 1 0-6Z',
  alert: 'M12 9v4m0 4h.01M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z',
  inbox: 'M4 13h4l2 3h4l2-3h4M4 13 6 5h12l2 8v6a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-6Z',
  info: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Zm0-13h.01M11 12h1v5h1',
} as const

export type IconName = keyof typeof ICON_PATHS

export interface IconProps {
  name: IconName
  className?: string
  title?: string
}

export function Icon({ name, className, title }: IconProps) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={cn('size-5 shrink-0', className)}
      aria-hidden={title ? undefined : 'true'}
      role={title ? 'img' : 'presentation'}
    >
      {title ? <title>{title}</title> : null}
      <path d={ICON_PATHS[name]} />
    </svg>
  )
}
