import { useMutation } from '@tanstack/react-query'
import { FileSpreadsheet, FileDown, Upload } from 'lucide-react'
import { useRef, useState } from 'react'
import { errorMessage } from '../../../api/client'
import type { ImportError, ImportResult } from '../../../api/types'
import { downloadUserImportTemplate, importUsers } from '../../../api/users'
import Button from '../../../components/ui/Button'
import Modal from '../../../components/ui/Modal'
import Table, { type Column } from '../../../components/ui/Table'
import { useToast } from '../../../components/ui/toast-context'
import { saveBlob } from '../../../utils/download'

interface Props {
  open: boolean
  onClose: () => void
  /** 至少有一条成功导入时回调（用于刷新列表） */
  onImported: () => void
}

const ACCEPT = '.xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
const MAX_SIZE = 10 * 1024 * 1024

const errorColumns: Column<ImportError>[] = [
  { key: 'row', title: '行号', width: 70, render: (e) => <span className="font-mono text-xs">{e.row}</span> },
  { key: 'username', title: '用户名', width: 140, render: (e) => e.username ?? '—' },
  { key: 'message', title: '失败原因', render: (e) => <span className="text-red-600">{e.message}</span> },
]

function ImportBody({ onClose, onImported }: Omit<Props, 'open'>) {
  const toast = useToast()
  const inputRef = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<ImportResult | null>(null)

  const template = useMutation({
    mutationFn: downloadUserImportTemplate,
    onSuccess: (f) => saveBlob(f.blob, f.filename),
    onError: (e) => toast.error('下载模板失败', errorMessage(e)),
  })

  const upload = useMutation({
    mutationFn: (f: File) => importUsers(f),
    onSuccess: (r) => {
      setResult(r)
      if (r.success > 0) onImported()
      if (r.failed === 0) toast.success('导入完成', `成功 ${r.success} 条`)
      else toast.warning('导入完成，部分失败', `成功 ${r.success} 条，失败 ${r.failed} 条`)
    },
    onError: (e) => toast.error('导入失败', errorMessage(e)),
  })

  const pick = (f: File | undefined) => {
    if (!f) return
    if (!/\.xlsx$/i.test(f.name)) {
      toast.error('仅支持 .xlsx 文件')
      return
    }
    if (f.size > MAX_SIZE) {
      toast.error('文件过大', '请控制在 10MB 以内')
      return
    }
    setFile(f)
    setResult(null)
  }

  return (
    <div className="space-y-4">
      <div className="rounded-lg bg-slate-50 px-3 py-2.5 text-xs text-slate-500 leading-relaxed">
        模板列：<span className="text-slate-700">用户名*、姓名*、手机号、邮箱、工号、部门名称、角色代码</span>（多个角色用逗号分隔）。
        逐行校验，失败行不影响其他行；导入用户的密码为系统缺省密码。
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Button variant="secondary" icon={FileDown} loading={template.isPending} onClick={() => template.mutate()}>
          下载模板
        </Button>
        <input
          ref={inputRef}
          type="file"
          accept={ACCEPT}
          className="hidden"
          onChange={(e) => {
            pick(e.target.files?.[0])
            e.target.value = ''
          }}
        />
        <Button variant="secondary" icon={FileSpreadsheet} onClick={() => inputRef.current?.click()}>
          选择文件
        </Button>
        {file && (
          <span className="text-sm text-slate-600 truncate max-w-[16rem]" title={file.name}>
            {file.name} <span className="text-slate-400">({Math.max(1, Math.round(file.size / 1024))} KB)</span>
          </span>
        )}
      </div>

      {result && (
        <div className="space-y-3">
          <div className="grid grid-cols-3 gap-2 text-center">
            <div className="rounded-lg bg-slate-50 py-2">
              <div className="text-lg font-semibold text-slate-800">{result.total}</div>
              <div className="text-xs text-slate-400">总计</div>
            </div>
            <div className="rounded-lg bg-emerald-50 py-2">
              <div className="text-lg font-semibold text-emerald-700">{result.success}</div>
              <div className="text-xs text-emerald-600">成功</div>
            </div>
            <div className="rounded-lg bg-red-50 py-2">
              <div className="text-lg font-semibold text-red-700">{result.failed}</div>
              <div className="text-xs text-red-600">失败</div>
            </div>
          </div>
          {result.errors.length > 0 && (
            <div className="max-h-64 overflow-y-auto rounded-lg border border-slate-100 px-3">
              <Table columns={errorColumns} data={result.errors} rowKey={(e) => `${e.row}-${e.username ?? ''}`} />
            </div>
          )}
        </div>
      )}

      <div className="flex justify-end gap-2 pt-1">
        <Button variant="secondary" onClick={onClose} disabled={upload.isPending}>
          {result ? '关闭' : '取消'}
        </Button>
        <Button icon={Upload} disabled={!file} loading={upload.isPending} onClick={() => file && upload.mutate(file)}>
          开始导入
        </Button>
      </div>
    </div>
  )
}

/** Excel 批量导入用户 */
export default function ImportModal({ open, onClose, onImported }: Props) {
  return (
    <Modal open={open} onClose={onClose} title="导入用户" size="lg">
      {open && <ImportBody onClose={onClose} onImported={onImported} />}
    </Modal>
  )
}
