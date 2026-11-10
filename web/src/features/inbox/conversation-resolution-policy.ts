import type { ChatContact } from '../../api/chat'

export function conversationHasUnfinishedTasks(contact: ChatContact) {
  return !!(contact.active_tasks || contact.queued_tasks || contact.review_tasks || contact.failed_tasks ||
    (contact.task_count || 0) > (contact.done_tasks || 0) + (contact.canceled_tasks || 0))
}

export function resolvableConversation(contact: ChatContact) {
  return (contact.has_messages === true || (contact.message_count || 0) > 0 || (contact.task_count || 0) > 0) &&
    !conversationHasUnfinishedTasks(contact)
}
