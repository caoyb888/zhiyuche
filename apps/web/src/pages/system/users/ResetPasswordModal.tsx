import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Check, Copy } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { errorMessage } from '../../../api/client'
import type { User } from '../../../api/types'
import { resetUserPassword } from '../../../api/users'
import Button from '../../../components/ui/Button'
import { Form, FormField } from '../../../components/ui/Form'
import Input from '../../../components/ui/Input'
import Modal from '../../../components/ui/Modal'
import { useToast } from '../../../components/ui/toast-context'
import { copyText } from '../../../utils/download'
import { optionalPassword } from './schemas'

const schema = z.object({ password: optionalPassword })
type FormValues = z.infer<typeof schema>

function ResetPasswordBody({ user, onClose }: { user: User; onClose: () => void }) {
  const toast = useToast()
  const [result, setResult] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const form = useForm<FormValues>({ resolver: zodResolver(schema), defaultValues: { password: '' } })

  const mutation = useMutation({
    mutationFn: (v: FormValues) => resetUserPassword(user.id, v.password || undefined),
    onSuccess: (r) => {
      setResult(r.password)
      toast.success('密码已重置', '该用户所有登录会话已失效')
    },
    onError: (e) => toast.error('重置失败', errorMessage(e)),
  })

  const copy = async () => {
    if (result === null) return
    const ok = await copyText(result)
    setCopied(ok)
    if (!ok) toast.error('复制失败，请手动复制')
  }

  if (result !== null) {
    return (
      <div className="space-y-4">
        <p className="text-sm text-slate-600">
          用户 <span className="font-medium text-slate-800">{user.name}</span>（{user.username}）的新密码：
        </p>
        <div className="flex items-center gap-2">
          <code className="flex-1 rounded-lg bg-slate-50 border border-slate-200 px-3 py-2 font-mono text-base text-slate-800 break-all select-all">{result}</code>
          <Button variant="secondary" icon={copied ? Check : Copy} onClick={() => void copy()}>
            {copied ? '已复制' : '复制'}
          </Button>
        </div>
        <p className="text-xs text-amber-700 bg-amber-50 rounded-lg px-3 py-2">明文密码仅显示一次，请立即告知用户；关闭后无法再次查看。</p>
        <div className="flex justify-end">
          <Button onClick={onClose}>关闭</Button>
        </div>
      </div>
    )
  }

  return (
    <Form form={form} onSubmit={(v) => mutation.mutate(v)}>
      <p className="text-sm text-slate-600">
        为 <span className="font-medium text-slate-800">{user.name}</span>（{user.username}）重置密码，重置后该用户需重新登录。
      </p>
      <FormField name="password" label="新密码" hint="留空则使用系统缺省密码；否则至少 8 位，含字母和数字">
        <Input type="text" autoComplete="off" placeholder="留空使用系统缺省密码" {...form.register('password')} />
      </FormField>
      <div className="flex justify-end gap-2 pt-2">
        <Button variant="secondary" onClick={onClose} disabled={mutation.isPending}>
          取消
        </Button>
        <Button type="submit" loading={mutation.isPending}>
          确认重置
        </Button>
      </div>
    </Form>
  )
}

/** 重置密码：可填新密码或留空；成功后展示明文并可复制 */
export default function ResetPasswordModal({ user, onClose }: { user: User | null; onClose: () => void }) {
  return (
    <Modal open={user !== null} onClose={onClose} title="重置密码" size="sm">
      {user && <ResetPasswordBody key={user.id} user={user} onClose={onClose} />}
    </Modal>
  )
}
