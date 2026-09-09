import type { Settlement, SettlementLine, SettlementLineKind } from '../../../api/types'
import { formatDateTime, formatMoney, formatNumber } from '../../../utils/format'
import { SETTLEMENT_LINE_KIND_LABEL, SETTLEMENT_STATUS_LABEL, lineQuantityUnit } from '../style'

export interface PrintOptions {
  /** 抬头（租户名） */
  tenantName: string
  /** 打印人 */
  printedBy: string
}

const KIND_ORDER: SettlementLineKind[] = ['trip', 'charge', 'penalty']

function esc(v: unknown): string {
  return String(v ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c] ?? c)
}

/** 按类型分组（行程 → 充电 → 罚金），组内按发生时间升序 */
export function groupLines(lines: SettlementLine[] | undefined): Array<{ kind: SettlementLineKind; lines: SettlementLine[]; subtotal: number }> {
  const all = lines ?? []
  return KIND_ORDER.map((kind) => {
    const list = all.filter((l) => l.kind === kind).sort((a, b) => a.occurred_at.localeCompare(b.occurred_at))
    return { kind, lines: list, subtotal: list.reduce((n, l) => n + l.amount, 0) }
  }).filter((g) => g.lines.length > 0)
}

function lineRows(lines: SettlementLine[]): string {
  return lines
    .map(
      (l) => `<tr>
  <td>${esc(l.ref_no || l.ref_id)}</td>
  <td>${esc(formatDateTime(l.occurred_at))}</td>
  <td>${esc(l.user_name || '—')}</td>
  <td>${esc(l.vehicle_plate || '—')}</td>
  <td class="num">${l.quantity === null || l.quantity === undefined ? '—' : esc(`${formatNumber(l.quantity, 1)} ${lineQuantityUnit(l.kind)}`)}</td>
  <td class="num">${esc(formatMoney(l.amount))}</td>
</tr>`,
    )
    .join('')
}

/** 结算单打印视图 HTML（样式参考方案 §7.5"公务用车行程结算单"） */
export function buildSettlementPrintHtml(s: Settlement, opts: PrintOptions): string {
  const target = s.dept_id ? `${s.dept_name ?? '部门'}（部门结算单）` : `${opts.tenantName}（企业汇总）`
  const groups = groupLines(s.lines)
  const usage = s.budget > 0 ? `${((s.total / s.budget) * 100).toFixed(1)}%` : '未设预算'
  const detail =
    groups.length === 0
      ? '<p class="muted">本期无明细行</p>'
      : groups
          .map(
            (g) => `<h3>${esc(SETTLEMENT_LINE_KIND_LABEL[g.kind])}（${g.lines.length} 笔）</h3>
<table class="lines">
<thead><tr><th>单号</th><th>日期</th><th>用车人</th><th>车牌</th><th class="num">数量</th><th class="num">金额（元）</th></tr></thead>
<tbody>${lineRows(g.lines)}</tbody>
<tfoot><tr><td colspan="5">小计</td><td class="num">${esc(formatMoney(g.subtotal))}</td></tr></tfoot>
</table>`,
          )
          .join('')

  return `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>${esc(opts.tenantName)} ${esc(s.period)} 结算单</title>
<style>
  @page { size: A4; margin: 16mm 14mm; }
  * { box-sizing: border-box; }
  body { font-family: "PingFang SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif; color: #1e293b; font-size: 12px; margin: 0; padding: 24px; }
  .sheet { max-width: 820px; margin: 0 auto; }
  h1 { font-size: 20px; text-align: center; letter-spacing: 2px; margin: 0 0 4px; }
  .sub { text-align: center; color: #64748b; font-size: 12px; margin-bottom: 16px; }
  .rule { border: 0; border-top: 2px solid #1e293b; margin: 12px 0; }
  .rule.thin { border-top: 1px dashed #94a3b8; }
  .meta { display: grid; grid-template-columns: 1fr 1fr; gap: 4px 24px; }
  .meta div { display: flex; gap: 8px; }
  .meta b { color: #475569; font-weight: 500; min-width: 5em; }
  table { width: 100%; border-collapse: collapse; margin-top: 6px; }
  th, td { padding: 6px 8px; text-align: left; border-bottom: 1px solid #e2e8f0; }
  th { background: #f1f5f9; font-weight: 600; color: #475569; font-size: 11px; }
  td.num, th.num { text-align: right; font-variant-numeric: tabular-nums; }
  .summary td.total { font-weight: 700; font-size: 14px; }
  tfoot td { font-weight: 600; background: #f8fafc; }
  h3 { font-size: 13px; margin: 16px 0 2px; color: #334155; }
  .muted { color: #94a3b8; }
  .footer { margin-top: 20px; color: #64748b; font-size: 11px; display: flex; justify-content: space-between; }
  .actions { text-align: right; margin-bottom: 12px; }
  .actions button { font: inherit; padding: 6px 14px; border: 1px solid #cbd5e1; border-radius: 6px; background: #fff; cursor: pointer; }
  .stamp { display: inline-block; padding: 2px 8px; border: 1px solid currentColor; border-radius: 4px; font-size: 11px; }
  .stamp.ok { color: #047857; }
  .stamp.draft { color: #b45309; }
  @media print { .actions { display: none; } body { padding: 0; } }
</style>
</head>
<body>
<div class="sheet">
  <div class="actions"><button type="button" onclick="window.print()">打印 / 存为 PDF</button></div>
  <h1>智御公务用车月度结算单</h1>
  <div class="sub">${esc(opts.tenantName)} · 结算期 ${esc(s.period)}</div>
  <hr class="rule">
  <div class="meta">
    <div><b>结算对象</b><span>${esc(target)}</span></div>
    <div><b>结算单号</b><span>${esc(s.id)}</span></div>
    <div><b>生成时间</b><span>${esc(formatDateTime(s.generated_at))}</span></div>
    <div><b>状态</b><span class="stamp ${s.status === 'confirmed' ? 'ok' : 'draft'}">${esc(SETTLEMENT_STATUS_LABEL[s.status])}</span></div>
    <div><b>确认时间</b><span>${esc(s.confirmed_at ? formatDateTime(s.confirmed_at) : '—')}</span></div>
    <div><b>确认人</b><span>${esc(s.confirmed_by_name || '—')}</span></div>
  </div>
  <hr class="rule thin">
  <table class="summary">
    <thead><tr><th>项目</th><th class="num">笔数</th><th class="num">金额（元）</th></tr></thead>
    <tbody>
      <tr><td>行程费用</td><td class="num">${s.trip_count}</td><td class="num">${esc(formatMoney(s.trip_cost))}</td></tr>
      <tr><td>充电费用</td><td class="num">${s.charge_count}</td><td class="num">${esc(formatMoney(s.charge_cost))}</td></tr>
      <tr><td>违规罚金</td><td class="num">—</td><td class="num">${esc(formatMoney(s.penalty))}</td></tr>
      <tr><td class="total">合计</td><td class="num"></td><td class="num total">¥ ${esc(formatMoney(s.total))}</td></tr>
      <tr><td>月度预算</td><td class="num"></td><td class="num">${esc(s.budget > 0 ? formatMoney(s.budget) : '未设')}</td></tr>
      <tr><td>预算使用率</td><td class="num"></td><td class="num">${esc(usage)}</td></tr>
    </tbody>
  </table>
  <hr class="rule thin">
  <h2 style="font-size:14px;margin:8px 0 0">费用明细</h2>
  ${detail}
  <hr class="rule">
  <div class="footer">
    <span>数据来源：系统自动生成，不可篡改</span>
    <span>打印人：${esc(opts.printedBy)} · ${esc(formatDateTime(new Date().toISOString()))}</span>
  </div>
</div>
</body>
</html>`
}

/** 在新窗口打开打印视图并触发浏览器打印（可存为 PDF）；弹窗被拦截时返回 false */
export function openSettlementPrint(s: Settlement, opts: PrintOptions): boolean {
  const win = window.open('', '_blank')
  if (!win) return false
  win.document.open()
  win.document.write(buildSettlementPrintHtml(s, opts))
  win.document.close()
  win.focus()
  // 等字体 / 样式就绪后再打印；某些浏览器 load 事件不会再触发，兜底延时
  const print = () => {
    try {
      win.print()
    } catch {
      // 用户可点击页内按钮
    }
  }
  if (win.document.readyState === 'complete') window.setTimeout(print, 300)
  else win.addEventListener('load', () => window.setTimeout(print, 300), { once: true })
  return true
}
