import { useCallback, useEffect, useState, type CSSProperties, type FormEvent } from 'react'
import { Search, UserRoundCheck, UsersRound } from 'lucide-react'
import { request } from '../../shared/api'
import { initial } from '../../shared/format'
import type { SocialUser } from '../../shared/types'
import type { SocialView } from '../../app/AppShell'

export function PeoplePage({ initialTab, onToast, onProfile }: { initialTab: SocialView; onToast: (message: string) => void; onProfile: (uid: number) => void }) {
  const [tab, setTab] = useState<SocialView>(initialTab)
  const [users, setUsers] = useState<SocialUser[]>([])
  const [query, setQuery] = useState('')
  const [searching, setSearching] = useState(false)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setTab(initialTab)
    setQuery('')
    setSearching(false)
  }, [initialTab])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const payload = await request<{ users: SocialUser[] }>(`/api/social/${tab}`)
      setUsers(payload.users || [])
      setSearching(false)
    } finally { setLoading(false) }
  }, [tab])

  useEffect(() => { load().catch(error => onToast(error.message)) }, [load, onToast])

  async function search(event: FormEvent) {
    event.preventDefault()
    setLoading(true)
    try {
      const payload = await request<{ users: SocialUser[] }>(`/api/users/search?q=${encodeURIComponent(query)}`)
      setUsers(payload.users || [])
      setSearching(true)
    } catch (error) { onToast((error as Error).message) } finally { setLoading(false) }
  }

  async function follow(person: SocialUser) {
    try {
      const payload = await request<{ user: SocialUser; message: string }>(`/api/users/${person.uid}/follow`, { method: person.following ? 'DELETE' : 'POST' })
      setUsers(current => current.map(item => item.uid === person.uid ? payload.user : item))
      onToast(payload.message)
    } catch (error) { onToast((error as Error).message) }
  }

  return <div className="people-page">
    <section className="page-heading"><div><p className="overline">关系</p><h1>好友与关注</h1><p className="heading-note">找到重要的人，保持恰到好处的联系。</p></div><UsersRound size={30} strokeWidth={1.4} /></section>
    <form className="people-tools" onSubmit={search}><Search size={17} /><input className="search-input" placeholder="输入 UID 或用户名" value={query} onChange={event => setQuery(event.target.value)} required /><button className="button primary" disabled={loading}>搜索</button></form>
    <div className="tabs social-tabs">
      <button className={tab === 'following' && !searching ? 'active' : ''} onClick={() => { setTab('following'); setQuery('') }}>我的关注</button>
      <button className={tab === 'friends' && !searching ? 'active' : ''} onClick={() => { setTab('friends'); setQuery('') }}>好友</button>
      {searching ? <button className="active search-result-tab" onClick={() => load()}>搜索结果 <b>{users.length}</b></button> : null}
    </div>
    {loading ? <div className="list-skeleton"><span /><span /><span /></div> : <div className="people-list">{users.map((person, index) => <article className="person-row" key={person.uid} style={{ '--item-index': index } as CSSProperties}><button className="person-main" onClick={() => onProfile(person.uid)}><span className="avatar">{initial(person.username)}</span><span><strong>{person.username}</strong><small>UID {person.uid}{person.bio ? ` · ${person.bio}` : ''}</small></span></button><div className="relationship-actions">{person.friend ? <span className="relationship-badge"><UserRoundCheck size={13} />好友</span> : null}{!person.is_self ? <button className={`button ${person.following ? 'secondary' : 'primary'} compact`} onClick={() => follow(person)}>{person.following ? '已关注' : '关注'}</button> : null}</div></article>)}</div>}
    {!loading && users.length === 0 ? <div className="empty-state"><UsersRound size={34} strokeWidth={1.2} /><h3>{searching ? '没有找到这个人' : tab === 'friends' ? '还没有好友' : '还没有关注'}</h3><p>{searching ? '检查 UID 或用户名后再试一次。' : '通过上方搜索找到其他用户。'}</p></div> : null}
  </div>
}
