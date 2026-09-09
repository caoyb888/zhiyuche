import { forwardRef } from 'react'
import Input, { type InputProps } from './Input'

export interface DateTimeInputProps extends Omit<InputProps, 'type' | 'value' | 'onChange'> {
  /** 本地时间 `YYYY-MM-DDTHH:mm`（空串表示未选）；与 RFC3339 的转换见 utils/format */
  value: string
  onChange: (value: string) => void
}

/** 原生 datetime-local 输入的统一外观封装（受控） */
const DateTimeInput = forwardRef<HTMLInputElement, DateTimeInputProps>(function DateTimeInput({ value, onChange, ...rest }, ref) {
  return <Input ref={ref} type="datetime-local" value={value} onChange={(e) => onChange(e.target.value)} {...rest} />
})

export default DateTimeInput
