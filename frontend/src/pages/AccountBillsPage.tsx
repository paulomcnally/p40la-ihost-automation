import { useCallback, useEffect, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { useI18nStore } from '../stores/i18nStore'
import { usePageTitle } from '../hooks/usePageTitle'
import { useToast } from '../components/Toast'
import { Icon } from '../components/Icons'
import Dropdown, { type DropdownOption } from '../components/Dropdown'
import { api, type AccountPlugin, type AccountWebhook, type BillItem, type BillRecord, type Plugin, type WebhookLog } from '../api'

const PAGE_SIZE = 5

function formatDate(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

function isToday(iso: string): boolean {
  if (!iso) return false
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return false
  const now = new Date()
  return d.getDate() === now.getDate() && d.getMonth() === now.getMonth() && d.getFullYear() === now.getFullYear()
}

function Toggle({ checked, onChange, disabled }: { checked: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      onClick={() => onChange(!checked)}
      disabled={disabled}
      className={`w-12 h-7 rounded-full relative transition-colors flex-shrink-0 disabled:opacity-50 ${checked ? 'bg-primary' : 'bg-border'}`}
    >
      <span className={`absolute top-1 w-5 h-5 rounded-full bg-white transition-all ${checked ? 'left-6' : 'left-1'}`} />
    </button>
  )
}

export default function AccountBillsPage() {
  const { accountId } = useParams()
  const account = Number(accountId)
  const { t } = useI18nStore()
  const { showToast } = useToast()
  usePageTitle(t('bills.title'))
  const [searchParams, setSearchParams] = useSearchParams()

  const [tab, setTab] = useState<'historico' | 'config'>(
    searchParams.get('tab') === 'config' ? 'config' : 'historico',
  )

  const [plugins, setPlugins] = useState<Plugin[]>([])
  const [accountPlugin, setAccountPlugin] = useState<AccountPlugin | null>(null)
  const [fetching, setFetching] = useState(false)
  const [settingPlugin, setSettingPlugin] = useState(false)

  const [records, setRecords] = useState<BillRecord[]>([])
  const [offset, setOffset] = useState(0)
  const [hasNewer, setHasNewer] = useState(false)
  const [loadingBills, setLoadingBills] = useState(true)

  const [webhook, setWebhook] = useState<AccountWebhook | null>(null)
  const [webhookURL, setWebhookURL] = useState('')
  const [scheduleTime, setScheduleTime] = useState('')
  const [webhookEnabled, setWebhookEnabled] = useState(false)
  const [scheduleFrequency, setScheduleFrequency] = useState('daily')
  const [scheduleInterval, setScheduleInterval] = useState('')
  const [scheduleDays, setScheduleDays] = useState('')
  const [savingWebhook, setSavingWebhook] = useState(false)
  const [testingWebhook, setTestingWebhook] = useState(false)
  const [webhookLogs, setWebhookLogs] = useState<WebhookLog[]>([])
  const [logsOffset, setLogsOffset] = useState(0)
  const [logsHasNewer, setLogsHasNewer] = useState(false)
  const [loadingConfig, setLoadingConfig] = useState(true)

  const loadBills = useCallback(async (off: number) => {
    if (!Number.isFinite(account)) return
    setLoadingBills(true)
    try {
      const res = await api.bills.list(account, PAGE_SIZE + 1, off)
      const all = res?.items ?? []
      setHasNewer(all.length > PAGE_SIZE)
      setRecords(all.slice(0, PAGE_SIZE))
      setOffset(off)
      const params = new URLSearchParams(searchParams)
      params.set('offset', String(off))
      params.delete('at')
      setSearchParams(params, { replace: true })
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    } finally {
      setLoadingBills(false)
    }
  }, [account, t, showToast, searchParams, setSearchParams])

  const jumpToAt = useCallback(async (at: string) => {
    if (!Number.isFinite(account)) return
    setTab('historico')
    setLoadingBills(true)
    try {
      const res = await api.bills.list(account, PAGE_SIZE + 1, 0, at)
      const all = res?.items ?? []
      setHasNewer(all.length > PAGE_SIZE)
      setRecords(all.slice(0, PAGE_SIZE))
      setOffset(res?.offset ?? 0)
      const params = new URLSearchParams(searchParams)
      params.set('tab', 'historico')
      params.set('offset', String(res?.offset ?? 0))
      params.delete('at')
      setSearchParams(params, { replace: true })
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    } finally {
      setLoadingBills(false)
    }
  }, [account, t, showToast, searchParams, setSearchParams])

  const loadConfig = useCallback(async () => {
    if (!Number.isFinite(account)) return
    setLoadingConfig(true)
    try {
      const [pluginList, acctPlugin, acctWebhook] = await Promise.all([
        api.plugins.list(),
        api.bills.accountPlugin(account),
        api.webhooks.account(account),
      ])
      setPlugins(pluginList ?? [])
      setAccountPlugin(acctPlugin ?? { plugin_name: null })
      setWebhook(acctWebhook ?? null)
      setWebhookURL(acctWebhook?.webhook_url ?? '')
      setScheduleTime(acctWebhook?.schedule_time ?? '')
      setWebhookEnabled(acctWebhook?.schedule_enabled ?? false)
      setScheduleFrequency(acctWebhook?.schedule_frequency ?? 'daily')
      setScheduleInterval(acctWebhook?.schedule_interval ? String(acctWebhook.schedule_interval) : '')
      setScheduleDays(acctWebhook?.schedule_days ?? '')
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    } finally {
      setLoadingConfig(false)
    }
  }, [account, t, showToast])

  const loadLogs = useCallback(async (off: number) => {
    if (!Number.isFinite(account)) return
    try {
      const logs = await api.webhooks.logs(account, PAGE_SIZE + 1, off)
      const all = logs ?? []
      setLogsHasNewer(all.length > PAGE_SIZE)
      setWebhookLogs(all.slice(0, PAGE_SIZE))
      setLogsOffset(off)
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    }
  }, [account, t, showToast])

  useEffect(() => {
    const at = searchParams.get('at')
    if (at) {
      jumpToAt(at)
    } else {
      loadBills(Number(searchParams.get('offset')) || 0)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
  useEffect(() => { loadConfig() }, [loadConfig])
  useEffect(() => { loadLogs(0) }, [loadLogs])

  const handleTabChange = (key: 'historico' | 'config') => {
    setTab(key)
    const params = new URLSearchParams(searchParams)
    params.set('tab', key)
    setSearchParams(params, { replace: true })
  }

  const goToLastRun = () => {
    if (!webhook?.last_run_at) return
    jumpToAt(webhook.last_run_at)
  }

  const handleSetPlugin = async (pluginName: string) => {
    setSettingPlugin(true)
    try {
      const updated = await api.bills.setAccountPlugin(account, pluginName || null)
      setAccountPlugin(updated ?? { plugin_name: null })
      showToast(t('bills.plugin_updated'), 'success')
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    } finally {
      setSettingPlugin(false)
    }
  }

  const handleFetch = async () => {
    setFetching(true)
    try {
      const result = await api.bills.fetch(account)
      showToast(
        result?.status === 'ok' ? t('bills.fetch_ok') : t('bills.fetch_error'),
        result?.status === 'ok' ? 'success' : 'error',
      )
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    } finally {
      setFetching(false)
      loadBills(0)
    }
  }

  const handleToggleWebhook = async (enabled: boolean) => {
    setWebhookEnabled(enabled)
    setSavingWebhook(true)
    try {
      await api.webhooks.setAccount(account, { schedule_enabled: enabled })
      showToast(t('webhooks.saved'), 'success')
      loadConfig()
    } catch (err: unknown) {
      setWebhookEnabled(!enabled)
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    } finally {
      setSavingWebhook(false)
    }
  }

  const handleSaveWebhook = async () => {
    setSavingWebhook(true)
    try {
      const interval = scheduleFrequency === 'every_n_days'
        ? (Number(scheduleInterval) > 0 ? Number(scheduleInterval) : null)
        : null
      const days = scheduleFrequency === 'weekly' || scheduleFrequency === 'monthly'
        ? (scheduleDays.trim() || null)
        : null
      const updated = await api.webhooks.setAccount(account, {
        webhook_url: webhookURL.trim() || null,
        schedule_time: scheduleTime.trim() || null,
        schedule_enabled: webhookEnabled,
        schedule_frequency: scheduleFrequency,
        schedule_interval: interval,
        schedule_days: days,
      })
      setWebhook(updated ?? null)
      showToast(t('webhooks.saved'), 'success')
      loadConfig()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    } finally {
      setSavingWebhook(false)
    }
  }

  const handleTestWebhook = async () => {
    setTestingWebhook(true)
    try {
      const result = await api.webhooks.test(account)
      showToast(
        `${t('webhooks.test_done')} ${t('webhooks.delivered')}: ${result?.delivered ?? 0}, ${t('webhooks.failed')}: ${result?.failed ?? 0}`,
        (result?.failed ?? 0) > 0 ? 'error' : 'success',
      )
      loadConfig()
      loadLogs(0)
      loadBills(0)
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    } finally {
      setTestingWebhook(false)
    }
  }

  const toggleDay = (day: number) => {
    const current = scheduleDays ? scheduleDays.split(',').map(Number).filter((d) => !Number.isNaN(d)) : []
    const next = current.includes(day) ? current.filter((d) => d !== day) : [...current, day]
    setScheduleDays(next.sort((a, b) => a - b).join(','))
  }

  const weekdays = [
    { value: 0, label: 'D' },
    { value: 1, label: 'L' },
    { value: 2, label: 'M' },
    { value: 3, label: 'X' },
    { value: 4, label: 'J' },
    { value: 5, label: 'V' },
    { value: 6, label: 'S' },
  ]

  const parseBills = (raw: string): BillItem[] => {
    try {
      const parsed = JSON.parse(raw)
      return Array.isArray(parsed) ? parsed : []
    } catch {
      return []
    }
  }

  const parsePayload = (raw: string): string => {
    try {
      return JSON.stringify(JSON.parse(raw), null, 2)
    } catch {
      return raw
    }
  }

  return (
    <div className="max-w-2xl mx-auto">
      <div className="flex flex-wrap items-center justify-between gap-3 mb-4">
        <h2 className="text-xl sm:text-2xl font-bold">{t('bills.title')}</h2>
        {tab === 'historico' && (
          <button
            onClick={handleFetch}
            disabled={fetching || !accountPlugin?.plugin_name}
            className="flex items-center gap-2 bg-primary text-white px-4 py-2 rounded-ios-sm font-medium hover:bg-primary-hover disabled:opacity-50 transition-colors min-h-[44px]"
          >
            <Icon name="refresh" className="w-4 h-4" />
            {fetching ? '...' : t('bills.fetch')}
          </button>
        )}
      </div>

      <div className="flex gap-1 bg-card border border-border rounded-ios-sm p-1 mb-6">
        {(['historico', 'config'] as const).map((key) => (
          <button
            key={key}
            onClick={() => handleTabChange(key)}
            className={`flex-1 py-2 px-3 rounded-ios-sm text-sm font-medium transition-colors min-h-[44px] ${
              tab === key ? 'bg-primary text-white' : 'text-text-secondary hover:bg-bg'
            }`}
          >
            {t(`bills.tab.${key}`)}
          </button>
        ))}
      </div>

      {tab === 'historico' ? (
        <>
          {webhook?.last_run_at && (
            <button
              onClick={goToLastRun}
              className="w-full flex items-center gap-2 bg-card border border-border rounded-ios-sm px-3 py-2 mb-4 text-sm text-text-secondary hover:bg-bg transition-colors min-h-[44px] text-left"
              title={t('bills.last_run_hint')}
            >
              <Icon name="info" className="w-4 h-4 text-primary flex-shrink-0" />
              <span>{t('bills.last_run')}: {formatDate(webhook.last_run_at)}</span>
            </button>
          )}
          {loadingBills ? (
            <div className="text-text-secondary py-10 text-center">{t('app.loading')}</div>
          ) : records.length === 0 ? (
            <div className="bg-card rounded-ios shadow-ios p-10 flex flex-col items-center justify-center text-center">
              <Icon name="bill" className="w-12 h-12 text-text-secondary/40 mb-3" />
              <p className="text-text-secondary">{t('bills.empty')}</p>
            </div>
          ) : (
            <>
              <ul className="space-y-3">
                {records.map((record) => {
                  const bills = record.status === 'ok' ? parseBills(record.raw) : []
                  return (
                    <li key={record.id}>
                      <div className="bg-card rounded-ios shadow-ios p-4">
                        <div className="flex items-center justify-between gap-3 flex-wrap">
                          <div className="flex items-center gap-2 flex-wrap">
                            {isToday(record.fetched_at) && (
                              <span className="text-xs px-2 py-0.5 rounded-ios-sm bg-primary/10 text-primary">
                                {t('bills.today')}
                              </span>
                            )}
                            <span className={`text-xs px-2 py-0.5 rounded-ios-sm ${
                              record.status === 'ok' ? 'bg-primary/10 text-primary' : 'bg-danger/10 text-danger'
                            }`}>
                              {t(`bills.status.${record.status}`)}
                            </span>
                            <span className="font-mono text-sm text-text-secondary">
                              {record.plugin_name} v{record.plugin_version}
                            </span>
                          </div>
                          <span className="text-xs text-text-secondary">{formatDate(record.fetched_at)}</span>
                        </div>
                        <p className="mt-1 text-xs text-text-secondary font-mono truncate">{record.source}</p>

                        {record.status === 'error' && record.error && (
                          <p className="mt-2 text-sm text-danger bg-danger/10 p-2 rounded-ios-sm">{record.error}</p>
                        )}

                        {bills.length > 0 && (
                          <ul className="mt-3 space-y-2">
                            {bills.map((bill, i) => (
                              <li key={i} className="flex items-center justify-between gap-3 bg-bg p-3 rounded-ios-sm">
                                <div className="min-w-0">
                                  <p className="font-medium truncate">
                                    {bill.invoice_number && (
                                      <span className="font-mono text-primary mr-1">{bill.invoice_number}</span>
                                    )}
                                    {bill.period || t('bills.period_unknown')}
                                  </p>
                                  <p className="text-sm text-text-secondary">
                                    {bill.due_date || t('bills.due_unknown')} · {bill.status || t('bills.status_unknown')}
                                  </p>
                                </div>
                                <span className="font-mono font-medium text-primary flex-shrink-0">{bill.amount}</span>
                              </li>
                            ))}
                          </ul>
                        )}
                      </div>
                    </li>
                  )
                })}
              </ul>

              <div className="flex items-center justify-between gap-3 mt-5">
                <button
                  onClick={() => loadBills(offset + PAGE_SIZE)}
                  className="px-4 py-2 rounded-ios-sm border border-border text-text-secondary hover:bg-bg transition-colors min-h-[44px]"
                >
                  ← {t('bills.older')}
                </button>
                <span className="text-sm text-text-secondary">{t('bills.page')} {Math.floor(offset / PAGE_SIZE) + 1}</span>
                <button
                  onClick={() => loadBills(Math.max(0, offset - PAGE_SIZE))}
                  disabled={offset === 0}
                  className="px-4 py-2 rounded-ios-sm border border-border text-text-secondary hover:bg-bg disabled:opacity-40 transition-colors min-h-[44px]"
                >
                  {t('bills.newer')} →
                </button>
              </div>

              <div className="mt-8">
                <h3 className="text-sm font-semibold mb-3">{t('webhooks.logs')}</h3>
                {webhookLogs.length === 0 ? (
                  <div className="bg-card rounded-ios shadow-ios p-6 text-center text-text-secondary">
                    {t('webhooks.logs_empty')}
                  </div>
                ) : (
                  <>
                    <ul className="space-y-3">
                      {webhookLogs.map((log) => (
                        <li key={log.id}>
                          <div className="bg-card rounded-ios shadow-ios p-4">
                            <div className="flex items-center justify-between gap-3 flex-wrap">
                              <div className="flex items-center gap-2 flex-wrap">
                                <span className={`text-xs px-2 py-0.5 rounded-ios-sm ${
                                  log.status === 'ok' ? 'bg-primary/10 text-primary' : 'bg-danger/10 text-danger'
                                }`}>
                                  {log.status === 'ok'
                                    ? (log.http_status ? `HTTP ${log.http_status}` : t('webhooks.status.ok'))
                                    : `${t('webhooks.status.error')}${log.http_status ? ` · HTTP ${log.http_status}` : ''}`}
                                </span>
                                <span className="text-xs text-text-secondary">{formatDate(log.ran_at)}</span>
                              </div>
                            </div>
                            <p className="mt-1 text-xs text-text-secondary font-mono truncate">{log.webhook_url}</p>
                            <pre className="mt-2 text-xs text-text-secondary whitespace-pre-wrap break-all font-mono">
                              {log.status === 'ok' ? parsePayload(log.response) : log.response}
                            </pre>
                          </div>
                        </li>
                      ))}
                    </ul>
                    <div className="flex items-center justify-between gap-3 mt-4">
                      <button
                        onClick={() => loadLogs(logsOffset + PAGE_SIZE)}
                        disabled={!logsHasNewer}
                        className="px-4 py-2 rounded-ios-sm border border-border text-text-secondary hover:bg-bg disabled:opacity-40 transition-colors min-h-[44px]"
                      >
                        ← {t('bills.older')}
                      </button>
                      <button
                        onClick={() => loadLogs(Math.max(0, logsOffset - PAGE_SIZE))}
                        disabled={logsOffset === 0}
                        className="px-4 py-2 rounded-ios-sm border border-border text-text-secondary hover:bg-bg disabled:opacity-40 transition-colors min-h-[44px]"
                      >
                        {t('bills.newer')} →
                      </button>
                    </div>
                  </>
                )}
              </div>
            </>
          )}
        </>
      ) : (
        <div className="space-y-6">
          <div className="bg-card rounded-ios shadow-ios p-4">
            <label className="block text-sm font-medium mb-1">{t('bills.plugin')}</label>
            <Dropdown
              value={accountPlugin?.plugin_name ?? ''}
              onChange={handleSetPlugin}
              disabled={settingPlugin}
              placeholder={t('bills.no_plugin')}
              options={[
                { value: '', label: t('bills.no_plugin') },
                ...plugins.map((p): DropdownOption => ({ value: p.name, label: `${p.name} (v${p.version})` })),
              ]}
            />
            {accountPlugin?.plugin_name && (
              <p className="mt-2 text-xs text-text-secondary">
                {t('bills.plugin_version')}: v{accountPlugin.version}
              </p>
            )}
            <p className="mt-1 text-xs text-text-secondary">{t('bills.plugin_hint')}</p>
          </div>

          <div className="bg-card rounded-ios shadow-ios p-4">
            <div className="flex items-center justify-between gap-3 mb-3">
              <div>
                <h3 className="font-semibold">{t('webhooks.account_title')}</h3>
                <p className="text-sm text-text-secondary">{t('webhooks.account_subtitle')}</p>
              </div>
              <Toggle checked={webhookEnabled} onChange={handleToggleWebhook} disabled={savingWebhook} />
            </div>
            {loadingConfig ? (
              <div className="text-text-secondary py-4 text-center">{t('app.loading')}</div>
            ) : (
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium mb-1">{t('webhooks.url')}</label>
                  <input
                    type="text"
                    value={webhookURL}
                    onChange={(e) => setWebhookURL(e.target.value)}
                    className="w-full bg-card text-text px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px] font-mono"
                    placeholder={t('webhooks.url_placeholder')}
                  />
                </div>
                <div className="flex flex-wrap items-end gap-3">
                  <div className="flex-1 min-w-[140px]">
                    <label className="block text-sm font-medium mb-1">{t('webhooks.frequency')}</label>
                    <Dropdown
                      value={scheduleFrequency}
                      onChange={setScheduleFrequency}
                      disabled={savingWebhook}
                      options={[
                        { value: 'daily', label: t('webhooks.freq.daily') },
                        { value: 'every_n_days', label: t('webhooks.freq.every_n_days') },
                        { value: 'weekly', label: t('webhooks.freq.weekly') },
                        { value: 'monthly', label: t('webhooks.freq.monthly') },
                      ]}
                    />
                  </div>
                  <div className="flex-1 min-w-[140px]">
                    <label className="block text-sm font-medium mb-1">{t('webhooks.schedule_time')}</label>
                    <input
                      type="time"
                      value={scheduleTime}
                      onChange={(e) => setScheduleTime(e.target.value)}
                      className="w-full bg-card text-text px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px]"
                    />
                  </div>
                  <div className="flex gap-2">
                    <button
                      onClick={handleTestWebhook}
                      disabled={testingWebhook || !accountPlugin?.plugin_name}
                      className="px-4 py-2 rounded-ios-sm border border-border text-text-secondary hover:bg-bg disabled:opacity-50 transition-colors min-h-[44px]"
                    >
                      {testingWebhook ? '...' : t('webhooks.test')}
                    </button>
                    <button
                      onClick={handleSaveWebhook}
                      disabled={savingWebhook}
                      className="px-4 py-2 rounded-ios-sm bg-primary text-white font-medium hover:bg-primary-hover disabled:opacity-50 transition-colors min-h-[44px]"
                    >
                      {savingWebhook ? '...' : t('app.save')}
                    </button>
                  </div>
                </div>

                {scheduleFrequency === 'every_n_days' && (
                  <div>
                    <label className="block text-sm font-medium mb-1">{t('webhooks.every_n_days')}</label>
                    <input
                      type="number"
                      min={1}
                      max={365}
                      value={scheduleInterval}
                      onChange={(e) => setScheduleInterval(e.target.value)}
                      className="w-full bg-card text-text px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px]"
                      placeholder="5"
                    />
                  </div>
                )}

                {scheduleFrequency === 'weekly' && (
                  <div>
                    <label className="block text-sm font-medium mb-1">{t('webhooks.weekly_days')}</label>
                    <div className="flex gap-1.5 flex-wrap">
                      {weekdays.map((day) => {
                        const active = scheduleDays.split(',').map(Number).includes(day.value)
                        return (
                          <button
                            key={day.value}
                            type="button"
                            onClick={() => toggleDay(day.value)}
                            className={`w-10 h-10 rounded-full text-sm font-medium transition-colors min-h-[44px] min-w-[44px] ${
                              active ? 'bg-primary text-white' : 'bg-bg text-text-secondary border border-border'
                            }`}
                          >
                            {day.label}
                          </button>
                        )
                      })}
                    </div>
                  </div>
                )}

                {scheduleFrequency === 'monthly' && (
                  <div>
                    <label className="block text-sm font-medium mb-1">{t('webhooks.monthly_day')}</label>
                    <input
                      type="number"
                      min={1}
                      max={31}
                      value={scheduleDays}
                      onChange={(e) => setScheduleDays(e.target.value)}
                      className="w-full bg-card text-text px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px]"
                      placeholder="15"
                    />
                  </div>
                )}

                {webhook?.last_run_at && (
                  <button
                    onClick={goToLastRun}
                    className="flex items-center gap-1.5 text-xs text-text-secondary hover:text-primary transition-colors min-h-[44px]"
                    title={t('bills.last_run_hint')}
                  >
                    <Icon name="info" className="w-3.5 h-3.5" />
                    {t('webhooks.last_run')}: {formatDate(webhook.last_run_at)}
                  </button>
                )}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}