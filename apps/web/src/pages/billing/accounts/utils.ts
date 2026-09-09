import type { AccountNode } from '../../../api/types'

/** 账户行是否已创建（树接口对未创建的部门 / 员工返回 exists=false、余额 0） */
export function nodeExists(n: AccountNode): boolean {
  return n.exists !== false && Boolean(n.id)
}

/** 树节点稳定键：已创建用账户 id，未创建用 级别:归属对象 id */
export function nodeKey(n: AccountNode): string {
  return nodeExists(n) ? n.id : `${n.level}:${n.owner_id}`
}
