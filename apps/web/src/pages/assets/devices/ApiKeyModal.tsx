import { Check, Copy } from 'lucide-react'
import { useState } from 'react'
import type { DeviceWithKey } from '../../../api/types'
import Button from '../../../components/ui/Button'
import Modal from '../../../components/ui/Modal'
import { useToast } from '../../../components/ui/toast-context'
import { copyText } from '../../../utils/download'

interface ApiKeyModalProps {
  /** 新建 / 换密钥返回的设备（含一次性 api_key） */
  device: DeviceWithKey | null
  /** 标题：新建 or 重置 */
  reason: 'create' | 'rotate'
  onClose: () => void
}

function ApiKeyBody({ device, reason, onClose }: { device: DeviceWithKey; reason: 'create' | 'rotate'; onClose: () => void }) {
  const toast = useToast()
  const [copied, setCopied] = useState(false)

  const copy = async () => {
    const ok = await copyText(device.api_key)
    setCopied(ok)
    if (!ok) toast.error('复制失败，请手动复制')
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-ink">
        设备 <span className="font-mono font-medium text-ink-strong">{device.serial_no}</span> 的{reason === 'rotate' ? '新' : ''}接入密钥（api_key）：
      </p>
      <div className="flex items-center gap-2">
        <code className="flex-1 select-all break-all rounded-lg border border-line-strong bg-surface-3 px-3 py-2 font-mono text-sm text-ink-strong">{device.api_key}</code>
        <Button variant="secondary" icon={copied ? Check : Copy} onClick={() => void copy()}>
          {copied ? '已复制' : '复制'}
        </Button>
      </div>
      <p className="rounded-lg bg-warn-500/10 px-3 py-2 text-xs text-warn-200">
        密钥仅显示这一次，请立即写入网关配置；关闭后无法再次查看。{reason === 'rotate' ? '旧密钥已立即失效。' : '网关上报时以 X-Device-Key 携带该密钥。'}
      </p>
      <div className="flex justify-end">
        <Button onClick={onClose}>我已保存</Button>
      </div>
    </div>
  )
}

/** 一次性展示 api_key；关闭前不允许点遮罩误关 */
export default function ApiKeyModal({ device, reason, onClose }: ApiKeyModalProps) {
  return (
    <Modal open={device !== null} onClose={onClose} title={reason === 'rotate' ? '密钥已重置' : '设备已创建'} size="md" closeOnBackdrop={false}>
      {device && <ApiKeyBody key={device.api_key} device={device} reason={reason} onClose={onClose} />}
    </Modal>
  )
}
