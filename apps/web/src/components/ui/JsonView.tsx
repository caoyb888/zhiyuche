import clsx from 'clsx'
import { useMemo, type ReactNode } from 'react'
import { stringifyJson } from '../../utils/format'

export interface JsonViewProps {
  /** 任意可序列化的值；null / undefined 显示 emptyText */
  value: unknown
  emptyText?: ReactNode
  /** 内容区最大高度（CSS 值），超出滚动 */
  maxHeight?: number | string
  className?: string
}

type TokenKind = 'key' | 'string' | 'number' | 'boolean' | 'null' | 'punct'

interface Token {
  kind: TokenKind
  text: string
}

const TOKEN_RE = /("(?:\\.|[^"\\])*")(\s*:)?|(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)|\b(true|false)\b|\b(null)\b|([{}[\],:])|(\s+)/g

/** 把 JSON 文本切成带类别的片段（只做展示，不求严格解析） */
function tokenize(text: string): Token[] {
  const out: Token[] = []
  let last = 0
  for (const m of text.matchAll(TOKEN_RE)) {
    const idx = m.index ?? 0
    if (idx > last) out.push({ kind: 'punct', text: text.slice(last, idx) })
    const [, str, colon, num, bool, nul, punct, ws] = m
    if (str !== undefined) {
      out.push({ kind: colon ? 'key' : 'string', text: str })
      if (colon) out.push({ kind: 'punct', text: colon })
    } else if (num !== undefined) out.push({ kind: 'number', text: num })
    else if (bool !== undefined) out.push({ kind: 'boolean', text: bool })
    else if (nul !== undefined) out.push({ kind: 'null', text: nul })
    else if (punct !== undefined) out.push({ kind: 'punct', text: punct })
    else if (ws !== undefined) out.push({ kind: 'punct', text: ws })
    last = idx + m[0].length
  }
  if (last < text.length) out.push({ kind: 'punct', text: text.slice(last) })
  return out
}

const classByKind: Record<TokenKind, string> = {
  key: 'text-violet-200',
  string: 'text-ev-200',
  number: 'text-brand-300',
  boolean: 'text-warn-200',
  null: 'text-ink-disabled italic',
  punct: 'text-ink-muted',
}

/** 只读 JSON 展示：缩进 + 轻量语法高亮 */
export default function JsonView({ value, emptyText = '—', maxHeight = '24rem', className }: JsonViewProps) {
  const tokens = useMemo(() => (value === null || value === undefined ? null : tokenize(stringifyJson(value))), [value])
  if (!tokens) {
    return <div className={clsx('rounded-lg bg-surface-3 border border-line px-3 py-6 text-center text-xs text-ink-faint', className)}>{emptyText}</div>
  }
  return (
    <pre
      className={clsx('overflow-auto rounded-lg bg-surface-3 border border-line px-3 py-2 font-mono text-xs leading-5 text-ink whitespace-pre-wrap break-all', className)}
      style={{ maxHeight }}
    >
      {tokens.map((t, i) => (
        <span key={i} className={classByKind[t.kind]}>
          {t.text}
        </span>
      ))}
    </pre>
  )
}
