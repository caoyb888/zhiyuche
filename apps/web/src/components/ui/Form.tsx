import clsx from 'clsx'
import { cloneElement, isValidElement, useId, type ReactNode } from 'react'
import { FormProvider, useFormContext, type FieldValues, type SubmitHandler, type UseFormReturn } from 'react-hook-form'

export interface FormProps<T extends FieldValues> {
  form: UseFormReturn<T>
  onSubmit: SubmitHandler<T>
  children: ReactNode
  className?: string
  /** 供外部按钮 `form={id}` 触发提交（如 Drawer footer） */
  id?: string
}

/** react-hook-form 表单容器：注入 FormProvider 并接管 submit */
export function Form<T extends FieldValues>({ form, onSubmit, children, className, id }: FormProps<T>) {
  return (
    <FormProvider {...form}>
      <form id={id} noValidate onSubmit={form.handleSubmit(onSubmit)} className={clsx('space-y-4', className)}>
        {children}
      </form>
    </FormProvider>
  )
}

export interface FieldRenderState {
  id: string
  invalid: boolean
  error?: string
}

interface InjectedProps {
  id?: string
  invalid?: boolean
}

export interface FormFieldProps {
  /** 字段名（支持 a.b 嵌套路径） */
  name: string
  label?: ReactNode
  /** 显示必填星号（仅展示，校验由 zod 负责） */
  required?: boolean
  /** 无错误时显示的提示 */
  hint?: ReactNode
  /**
   * 单个输入元素（自动注入 id / invalid）；
   * 或 render 函数（用于 Controller 包裹的受控组件）
   */
  children: ReactNode | ((state: FieldRenderState) => ReactNode)
  className?: string
}

/** 从 errors 对象按路径取出错误信息 */
function readError(errors: unknown, name: string): string | undefined {
  let cur: unknown = errors
  for (const part of name.split('.')) {
    if (typeof cur !== 'object' || cur === null) return undefined
    cur = (cur as Record<string, unknown>)[part]
  }
  if (typeof cur !== 'object' || cur === null) return undefined
  const msg = (cur as { message?: unknown }).message
  return typeof msg === 'string' && msg ? msg : undefined
}

/** 表单项：label、必填标记、错误提示；必须在 <Form> 内使用 */
export function FormField({ name, label, required, hint, children, className }: FormFieldProps) {
  const autoId = useId()
  const id = `f-${autoId}`
  const {
    formState: { errors },
  } = useFormContext()
  const error = readError(errors, name)
  const invalid = error !== undefined

  let control: ReactNode
  if (typeof children === 'function') {
    control = children({ id, invalid, error })
  } else if (isValidElement<InjectedProps>(children)) {
    control = cloneElement(children, { id: children.props.id ?? id, invalid: children.props.invalid ?? invalid })
  } else {
    control = children
  }

  return (
    <div className={clsx('space-y-1.5', className)}>
      {label && (
        <label htmlFor={id} className="block text-sm font-medium text-slate-700">
          {label}
          {required && <span className="ml-0.5 text-red-500">*</span>}
        </label>
      )}
      {control}
      {error ? (
        <p className="text-xs text-red-500" role="alert">
          {error}
        </p>
      ) : hint ? (
        <p className="text-xs text-slate-400">{hint}</p>
      ) : null}
    </div>
  )
}
