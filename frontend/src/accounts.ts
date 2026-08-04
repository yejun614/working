import type { Account } from '../bindings/working/internal/modules/account/types/models'

// canSendMail은 이 계정으로 메일을 보낼 수 있는지 판단한다.
// Google OAuth 계정은 Gmail API로 발송하므로 SMTP 설정이 없어도 보낼 수 있다.
export function canSendMail(account: Account | null | undefined): boolean {
  if (!account?.mail) return false
  return account.authType === 'oauth2' || !!account.mail.smtp
}
