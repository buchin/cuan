import type { ChatContact } from '../../api/chat'

export function conversationHasUnfinishedTasks(contact: ChatContact) {
  return !!(contact.active_tasks || contact.queued_tasks || contact.review_tasks || contact.failed_tasks ||
    (contact.task_count || 0) > (contact.done_tasks || 0) + (contact.canceled_tasks || 0))
}

export function completedConversation(contact: ChatContact) {
  const total = contact.task_count || 0
  return total > 0 && (contact.done_tasks || 0) > 0 &&
    (contact.done_tasks || 0) + (contact.canceled_tasks || 0) === total &&
    !(contact.active_tasks || contact.queued_tasks || contact.review_tasks || contact.failed_tasks)
}

