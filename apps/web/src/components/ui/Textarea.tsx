import { forwardRef, type TextareaHTMLAttributes } from 'react'
import clsx from 'clsx'
import { controlClass } from './styles'

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  invalid?: boolean
}

const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { invalid, className, rows = 3, ...rest },
  ref,
) {
  return (
    <textarea
      ref={ref}
      rows={rows}
      aria-invalid={invalid || undefined}
      className={controlClass(invalid, clsx('px-3 py-2 resize-y min-h-[2.25rem]', className))}
      {...rest}
    />
  )
})

export default Textarea
