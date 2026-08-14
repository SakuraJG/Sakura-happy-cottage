import { useState, type FormEvent } from 'react'
import { request } from '../../shared/api'
import type { SystemInfo, User } from '../../shared/types'

export function AccountPanel({ user, onToast }: { user: User; onToast: (message: string) => void }) {
  const [email, setEmail] = useState(user.email || ''); const [emailPassword, setEmailPassword] = useState('')
  const [currentPassword, setCurrentPassword] = useState(''); const [nextPassword, setNextPassword] = useState('')
  const [system, setSystem] = useState<SystemInfo | null>(null)

  async function bindEmail(event: FormEvent) { event.preventDefault(); try { const payload = await request<{ message: string }>('/api/account/email', { method: 'POST', body: JSON.stringify({ email, password: emailPassword }) }); setEmailPassword(''); onToast(payload.message) } catch (error) { onToast((error as Error).message) } }
  async function changePassword(event: FormEvent) { event.preventDefault(); try { const payload = await request<{ message: string }>('/api/account/password', { method: 'POST', body: JSON.stringify({ current_password: currentPassword, new_password: nextPassword }) }); setCurrentPassword(''); setNextPassword(''); onToast(payload.message) } catch (error) { onToast((error as Error).message) } }
  async function loadSystem() { try { setSystem(await request<SystemInfo>('/api/admin/system')) } catch (error) { onToast((error as Error).message) } }
  async function saveSystem(event: FormEvent) {
    event.preventDefault()
    if (!system) return
    const form = new FormData(event.currentTarget as HTMLFormElement)
    const settings = system.settings
    try {
      const payload = await request<{ message: string; settings: SystemInfo['settings'] }>('/api/admin/system', {
        method: 'PUT',
        body: JSON.stringify({
          registration_enabled: settings.registration_enabled,
          email_confirmation_required: settings.email_confirmation_required,
          password_recovery_enabled: settings.password_recovery_enabled,
          public_url: settings.public_url,
          smtp_enabled: settings.smtp_enabled,
          smtp_host: settings.smtp_host,
          smtp_port: settings.smtp_port,
          smtp_username: settings.smtp_username,
          smtp_password: form.get('smtp_password') || '',
          smtp_from_name: settings.smtp_from_name,
          smtp_from_email: settings.smtp_from_email,
          smtp_encryption: settings.smtp_encryption,
        }),
      })
      setSystem(current => current ? { ...current, settings: payload.settings } : current)
      onToast(payload.message)
    } catch (error) { onToast((error as Error).message) }
  }

  return <div className="account-layout">
    <form className="settings-section" onSubmit={bindEmail}><h2>绑定邮箱</h2><p>{user.email_verified ? `已绑定 ${user.email}` : '绑定后可用于登录和找回密码。'}</p><label>邮箱<input type="email" value={email} onChange={event => setEmail(event.target.value)} required /></label><label>当前密码<input type="password" value={emailPassword} onChange={event => setEmailPassword(event.target.value)} required /></label><button className="button secondary">绑定邮箱</button></form>
    <form className="settings-section" onSubmit={changePassword}><h2>修改密码</h2><p>修改后其他设备需要重新登录。</p><label>当前密码<input type="password" value={currentPassword} onChange={event => setCurrentPassword(event.target.value)} required /></label><label>新密码<input type="password" value={nextPassword} onChange={event => setNextPassword(event.target.value)} minLength={8} required /></label><button className="button secondary">修改密码</button></form>
    {user.role === 'admin' ? <section className="settings-section admin-settings"><div className="settings-title"><div><h2>系统设置</h2><p>配置注册、邮件与公开访问地址。</p></div>{!system ? <button className="button secondary" onClick={loadSystem}>加载设置</button> : null}</div>{system ? <form onSubmit={saveSystem}><div className="system-stats"><div><strong>{system.users}</strong><span>用户</span></div><div><strong>{system.memos}</strong><span>备忘录</span></div><div><strong>{Math.floor(system.uptime_seconds / 60)}</strong><span>运行分钟</span></div></div><div className="toggle-grid"><label><input type="checkbox" checked={system.settings.registration_enabled} onChange={event => setSystem({ ...system, settings: { ...system.settings, registration_enabled: event.target.checked } })} /> 开放注册</label><label><input type="checkbox" checked={system.settings.smtp_enabled} onChange={event => setSystem({ ...system, settings: { ...system.settings, smtp_enabled: event.target.checked } })} /> 启用 SMTP</label><label><input type="checkbox" checked={system.settings.email_confirmation_required} onChange={event => setSystem({ ...system, settings: { ...system.settings, email_confirmation_required: event.target.checked } })} /> 邮箱确认</label><label><input type="checkbox" checked={system.settings.password_recovery_enabled} onChange={event => setSystem({ ...system, settings: { ...system.settings, password_recovery_enabled: event.target.checked } })} /> 找回密码</label></div><label>公开访问地址<input value={system.settings.public_url} onChange={event => setSystem({ ...system, settings: { ...system.settings, public_url: event.target.value } })} required /></label><div className="field-grid"><label>SMTP 服务器<input value={system.settings.smtp_host} onChange={event => setSystem({ ...system, settings: { ...system.settings, smtp_host: event.target.value } })} /></label><label>端口<input type="number" min={1} max={65535} value={system.settings.smtp_port} onChange={event => setSystem({ ...system, settings: { ...system.settings, smtp_port: Number(event.target.value) } })} /></label></div><div className="field-grid"><label>用户名<input value={system.settings.smtp_username} onChange={event => setSystem({ ...system, settings: { ...system.settings, smtp_username: event.target.value } })} /></label><label>密码<input name="smtp_password" type="password" placeholder={system.settings.smtp_password_set ? '留空保留现有密码' : ''} /></label></div><div className="field-grid"><label>发件人<input value={system.settings.smtp_from_name} onChange={event => setSystem({ ...system, settings: { ...system.settings, smtp_from_name: event.target.value } })} /></label><label>发件邮箱<input type="email" value={system.settings.smtp_from_email} onChange={event => setSystem({ ...system, settings: { ...system.settings, smtp_from_email: event.target.value } })} /></label></div><label>加密<select value={system.settings.smtp_encryption} onChange={event => setSystem({ ...system, settings: { ...system.settings, smtp_encryption: event.target.value as SystemInfo['settings']['smtp_encryption'] } })}><option value="starttls">STARTTLS</option><option value="tls">TLS</option><option value="none">无加密</option></select></label><button className="button primary">保存系统设置</button></form> : null}</section> : null}
  </div>
}
