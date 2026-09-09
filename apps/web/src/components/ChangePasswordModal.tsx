import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { changePassword } from '../api/auth'
import { errorMessage } from '../api/client'
import Button from './ui/Button'
import { Form, FormField } from './ui/Form'
import Input from './ui/Input'
import Modal from './ui/Modal'
import { useToast } from './ui/toast-context'

const PASSWORD_RULE = /^(?=.*[A-Za-z])(?=.*\d).{8,}$/

const schema = z
  .object({
    old_password: z.string().min(1, '请输入原密码'),
    new_password: z.string().regex(PASSWORD_RULE, '至少 8 位，且包含字母和数字'),
    confirm: z.string().min(1, '请再次输入新密码'),
  })
  .refine((v) => v.new_password === v.confirm, { path: ['confirm'], message: '两次输入的新密码不一致' })

type FormValues = z.infer<typeof schema>

interface Props {
  open: boolean
  onClose: () => void
}

function ChangePasswordForm({ onClose }: { onClose: () => void }) {
  const toast = useToast()
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { old_password: '', new_password: '', confirm: '' },
  })

  const mutation = useMutation({
    mutationFn: (v: FormValues) => changePassword({ old_password: v.old_password, new_password: v.new_password }),
    onSuccess: () => {
      toast.success('密码已修改', '其他设备的登录状态已失效')
      onClose()
    },
    onError: (e) => toast.error('修改失败', errorMessage(e)),
  })

  return (
    <Form form={form} onSubmit={(v) => mutation.mutate(v)}>
      <FormField name="old_password" label="原密码" required>
        <Input type="password" autoComplete="current-password" {...form.register('old_password')} />
      </FormField>
      <FormField name="new_password" label="新密码" required hint="至少 8 位，包含字母和数字">
        <Input type="password" autoComplete="new-password" {...form.register('new_password')} />
      </FormField>
      <FormField name="confirm" label="确认新密码" required>
        <Input type="password" autoComplete="new-password" {...form.register('confirm')} />
      </FormField>
      <div className="flex justify-end gap-2 pt-2">
        <Button variant="secondary" onClick={onClose} disabled={mutation.isPending}>
          取消
        </Button>
        <Button type="submit" loading={mutation.isPending}>
          确认修改
        </Button>
      </div>
    </Form>
  )
}

/** 修改本人密码弹窗；每次打开重新挂载表单，避免残留输入 */
export default function ChangePasswordModal({ open, onClose }: Props) {
  return (
    <Modal open={open} onClose={onClose} title="修改密码" size="sm">
      {open && <ChangePasswordForm onClose={onClose} />}
    </Modal>
  )
}
