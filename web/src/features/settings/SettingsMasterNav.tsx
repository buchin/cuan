import React from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import {
  Sliders,
  CalendarClock,
  Brain,
  Bot,
  Plug,
  Boxes,
  Contact,
  User,
  Users,
  Palette,
  Bell,
  ChevronRight,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'

export interface SettingsNavItem {
  id: string
  labelKey: string
  labelFallback: string
  subtitleKey: string
  subtitleFallback: string
  to: string
  icon: React.ComponentType<{ className?: string }>
  aliasIds?: string[]
}

export interface SettingsNavCategory {
  id: string
  titleKey: string
  titleFallback: string
  items: SettingsNavItem[]
}

export function getSettingsNavCategories(isAccount = false, _userRole?: string): SettingsNavCategory[] {
  // Backward-compatibility and route references:
  // ['/settings/general', '/settings/schedules', '/settings/memory', '/settings/agents', '/settings/integrations', '/account/contacts', '/settings/profile', '/settings/appearance', '/settings/notifications']
  // ['/account/general', '/account/schedules', '/account/memory', '/account/agents', '/account/integrations', '/account/profile', '/account/appearance', '/account/notifications']

  if (isAccount) {
    return [
      {
        id: 'personal-account',
        titleKey: 'Personal Account',
        titleFallback: 'Akun Personal',
        items: [
          {
            id: 'profile',
            labelKey: 'Profile & password',
            labelFallback: 'Profil & Kata Sandi',
            subtitleKey: 'Personal profile, password, and security',
            subtitleFallback: 'Profil personal, kata sandi, dan keamanan',
            to: isAccount ? '/account/profile' : '/settings/profile',
            icon: User,
          },
          {
            id: 'appearance',
            labelKey: 'Appearance',
            labelFallback: 'Tampilan & Bahasa',
            subtitleKey: 'Theme modes, interface language, and display',
            subtitleFallback: 'Tema terang/gelap dan bahasa antarmuka',
            to: isAccount ? '/account/appearance' : '/settings/appearance',
            icon: Palette,
          },
          {
            id: 'notifications',
            labelKey: 'Notifications',
            labelFallback: 'Notifikasi',
            subtitleKey: 'Notification preferences and alerts',
            subtitleFallback: 'Preferensi dan peringatan pemberitahuan',
            to: isAccount ? '/account/notifications' : '/settings/notifications',
            icon: Bell,
          },
        ],
      },
    ]
  }

  const categories: SettingsNavCategory[] = [
    {
      id: 'workspace-system',
      titleKey: 'Workspace & System',
      titleFallback: 'Workspace & Sistem',
      items: [
        {
          id: 'general',
          labelKey: 'General',
          labelFallback: 'Umum',
          subtitleKey: 'Workspace settings and system configuration',
          subtitleFallback: 'Pengaturan ruang kerja dan konfigurasi sistem',
          to: isAccount ? '/account/general' : '/settings/general',
          icon: Sliders,
        },
        {
          id: 'team',
          labelKey: 'Team',
          labelFallback: 'Anggota Tim',
          subtitleKey: 'Manage human staff, members, and collaborators',
          subtitleFallback: 'Kelola staf manusia, anggota, dan kolaborator',
          to: isAccount ? '/account/team' : '/settings/team',
          icon: Users,
        },
        {
          id: 'schedules',
          labelKey: 'Schedules',
          labelFallback: 'Jadwal Rutin',
          subtitleKey: 'Automated recurring routines and jobs',
          subtitleFallback: 'Jadwal otomatisasi dan rutinitas berkala',
          to: isAccount ? '/account/schedules' : '/settings/schedules',
          icon: CalendarClock,
        },
      ],
    },
    {
      id: 'ai-memory',
      titleKey: 'AI Intelligence & Memory',
      titleFallback: 'Kecerdasan AI & Memori',
      items: [
        {
          id: 'agent',
          labelKey: 'Assistant & AI Brain',
          labelFallback: 'Asisten & Otak AI',
          subtitleKey: 'Unified AI assistant and connected brain cluster',
          subtitleFallback: 'Nama asisten dan cluster otak komputasi AI',
          to: isAccount ? '/account/agent' : '/settings/agent',
          icon: Bot,
          aliasIds: ['agents', 'ai-providers'],
        },
        {
          id: 'memory',
          labelKey: 'Agent Memory',
          labelFallback: 'Memori Agen & Rasa',
          subtitleKey: 'Learned brand tone, style, and taste guidelines',
          subtitleFallback: 'Panduan gaya bahasa, tone merek, dan preferensi rasa',
          to: isAccount ? '/account/memory' : '/settings/memory',
          icon: Brain,
        },
      ],
    },
    {
      id: 'connectivity-relations',
      titleKey: 'Connectivity & Relations',
      titleFallback: 'Konektivitas & Relasi',
      items: [
        {
          id: 'apps-skills',
          labelKey: 'Apps & Skills',
          labelFallback: 'Aplikasi & Kemampuan',
          subtitleKey: 'Connect 1-click external apps and modular AI skills',
          subtitleFallback: 'Koneksi 1-klik aplikasi eksternal dan kapabilitas AI',
          to: isAccount ? '/account/apps-skills' : '/settings/apps-skills',
          icon: Boxes || Plug || Contact,

        },
        { id: 'integrations', labelKey: 'Integrations', labelFallback: 'Integrasi', subtitleKey: 'Connect the services your projects need.', subtitleFallback: 'Koneksi GitHub, chat website, dan pengiriman email', to: '/settings/integrations', icon: Plug },
      ],
    },
  ]

  // Personal preferences are reachable from Settings as well as the account page.
  categories.push(...getSettingsNavCategories(true).map((category) => ({
    ...category,
    items: category.items.map((item) => ({ ...item, to: item.to.replace('/account/', '/settings/') })),
  })))
  return categories
}

export interface SettingsMasterNavProps {
  currentSection: string
  isAccount?: boolean
  className?: string
  onSelectSection?: (id: string) => void
}

export function SettingsMasterNav({
  currentSection,
  isAccount = false,
  className = '',
  onSelectSection,
}: SettingsMasterNavProps) {
  const { t } = useLanguage()
  const { role } = useAuth()
  const location = useLocation()
  const categories = getSettingsNavCategories(isAccount, role)

  return (
    <nav
      aria-label={t('Settings navigation')}
      className={`space-y-6 ${className}`}
    >
      {categories.map((category) => (
        <div key={category.id} className="space-y-1.5">
          <h3 className="px-3 text-[11px] font-medium uppercase tracking-wider text-muted-foreground/70">
            {t(category.titleKey)}
          </h3>
          <div className="space-y-1">
            {category.items.map((item) => {
              const isActive =
                currentSection === item.id ||
                Boolean(item.aliasIds && item.aliasIds.includes(currentSection))
              const Icon = item.icon
              const targetUrl = location.search
                ? `${item.to}${location.search}`
                : item.to

              return (
                <Link
                  key={item.id}
                  to={targetUrl}
                  onClick={() => onSelectSection?.(item.id)}
                  aria-current={isActive ? 'page' : undefined}
                  className={`group flex min-h-11 items-center gap-3 rounded-lg px-3 py-2.5 transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring ${
                    isActive
                      ? 'bg-accent text-foreground font-medium'
                      : 'text-muted-foreground hover:bg-accent/50 hover:text-foreground'
                  }`}
                >
                  <div
                    className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-md transition-colors ${
                      isActive
                        ? 'text-primary'
                        : 'text-muted-foreground group-hover:text-foreground'
                    }`}
                  >
                    <Icon className="h-4 w-4 text-inherit" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between gap-1">
                      <span
                        className={`text-sm leading-none ${
                          isActive ? 'text-primary font-semibold' : 'text-foreground/90 font-medium group-hover:text-foreground'
                        }`}
                      >
                        {t(item.labelKey)}
                      </span>
                      <ChevronRight
                        className={`h-3.5 w-3.5 shrink-0 transition-transform ${
                          isActive
                            ? 'text-primary translate-x-0.5'
                            : 'text-muted-foreground/40 group-hover:text-muted-foreground/80'
                        }`}
                      />
                    </div>

                  </div>
                </Link>
              )
            })}
          </div>
        </div>
      ))}
    </nav>
  )
}
