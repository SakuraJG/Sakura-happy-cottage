import { useEffect, useState, type FormEvent } from 'react'
import { ArrowLeft, CalendarDays, Settings, UserRoundCheck, UsersRound } from 'lucide-react'
import { request } from '../../shared/api'
import { initial } from '../../shared/format'
import type { SocialUser, User } from '../../shared/types'
import type { SocialView } from '../../app/AppShell'

export function ProfilePage({ uid, user, onUser, onToast, onBack, onSocial, onAccount }: {
  uid: number
  user: User
  onUser: (user: User) => void
  onToast: (message: string) => void
  onBack: () => void
  onSocial: (view: SocialView) => void
  onAccount: () => void
}) {
  const [person, setPerson] = useState<SocialUser | null>(null)
  const [username, setUsername] = useState(user.username)
  const [bio, setBio] = useState(user.bio || '')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    setPerson(null)
    request<{ user: SocialUser }>(`/api/users/${uid}`)
      .then(payload => {
        setPerson(payload.user)
        if (payload.user.is_self) {
          setUsername(payload.user.username)
          setBio(payload.user.bio || '')
        }
      })
      .catch(error => onToast(error.message))
  }, [uid, onToast])

  async function saveProfile(event: FormEvent) {
    event.preventDefault()
    setSaving(true)
    try {
      const payload = await request<{ user: User; message: string }>('/api/account/profile', {
        method: 'PATCH', body: JSON.stringify({ username, bio }),
      })
      onUser(payload.user)
      setPerson(current => current ? { ...current, username: payload.user.username, bio: payload.user.bio } : current)
      onToast(payload.message)
    } catch (error) { onToast((error as Error).message) } finally { setSaving(false) }
  }

  async function toggleFollow() {
    if (!person || person.is_self) return
    try {
      const payload = await request<{ user: SocialUser; message: string }>(`/api/users/${person.uid}/follow`, { method: person.following ? 'DELETE' : 'POST' })
      setPerson(payload.user)
      onToast(payload.message)
    } catch (error) { onToast((error as Error).message) }
  }

  if (!person) return <div className="profile-skeleton" aria-label="正在读取个人空间"><span /><span /><span /></div>

  const joined = new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long' }).format(new Date(person.created_at))

  return <div className="profile-page">
    <button className="back-button" onClick={onBack}><ArrowLeft size={15} />返回</button>
    <header className="profile-header">
      <span className="avatar profile-avatar">{initial(person.username)}</span>
      <div className="profile-title"><p className="overline">{person.is_self ? '我的个人空间' : '个人空间'}</p><h1>{person.username}</h1><span className="muted">UID {person.uid}</span></div>
      <div className="profile-actions">
        {person.is_self ? <button className="button secondary" onClick={onAccount}><Settings size={15} />账户与安全</button> : <button className={`button ${person.following ? 'secondary' : 'primary'}`} onClick={toggleFollow}>{person.following ? <UserRoundCheck size={15} /> : <UsersRound size={15} />}{person.following ? '已关注' : '关注'}</button>}
      </div>
    </header>
    <div className="profile-stats">
      <button disabled={!person.is_self} onClick={() => onSocial('following')}><strong>{person.following_count}</strong><span>关注</span></button>
      <div><strong>{person.follower_count}</strong><span>关注者</span></div>
      <button disabled={!person.is_self} onClick={() => onSocial('friends')}><strong>{person.friend_count}</strong><span>好友</span></button>
    </div>
    <section className="profile-section profile-about"><div><h2>关于</h2><p>{person.bio || '这个人还没有填写个人简介。'}</p></div><span className="joined"><CalendarDays size={14} />{joined} 加入</span></section>
    {person.is_self ? <form className="profile-editor" onSubmit={saveProfile}>
      <div className="section-heading"><div><h2>编辑个人信息</h2><p>用户名会展示给其他用户，账户名仅用于登录。</p></div></div>
      <label>用户名<input value={username} onChange={event => setUsername(event.target.value)} minLength={2} maxLength={32} required /></label>
      <label>个人简介<textarea value={bio} onChange={event => setBio(event.target.value)} maxLength={160} rows={4} /></label>
      <div className="profile-editor-footer"><span>{bio.length} / 160</span><button className="button primary" disabled={saving}>{saving ? '保存中…' : '保存个人信息'}</button></div>
    </form> : null}
  </div>
}
