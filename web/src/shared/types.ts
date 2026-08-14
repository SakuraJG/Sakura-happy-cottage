export type User = { id:number; uid:number; account:string; username:string; bio:string; role:string; must_change_password:boolean; email?:string; email_verified:boolean; created_at:string }
export type Attachment = { id:number; original_name:string; content_type:string; size:number }
export type Memo = { id:number; title:string; description:string; status:'open'|'done'; created_at:string; completed_at?:string; updated_at:string; attachments:Attachment[] }
export type SocialUser = { uid:number; username:string; bio:string; created_at:string; following:boolean; followed_by:boolean; friend:boolean; is_self:boolean; following_count:number; follower_count:number; friend_count:number }
export type SystemSettings = { registration_enabled:boolean; email_confirmation_required:boolean; password_recovery_enabled:boolean; public_url:string; smtp_enabled:boolean; smtp_host:string; smtp_port:number; smtp_username:string; smtp_password_set:boolean; smtp_from_name:string; smtp_from_email:string; smtp_encryption:'none'|'starttls'|'tls' }
export type SystemInfo = { users:number; memos:number; uptime_seconds:number; settings:SystemSettings }
