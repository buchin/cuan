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
  ChevronLeft,
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
          aliasIds: ['integrations'],
        },
        // Backward compatibility routes: id: 'integrations', id: 'contacts'
      ],
    },
  ]

  return categories
}

export interface SettingsMasterNavProps {
  currentSection: string
  isAccount?: boolean
  safeReturn?: string
  className?: string
  onSelectSection?: (id: string) => void
}

export function SettingsMasterNav({
  currentSection,
  isAccount = false,
  safeReturn = '',
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
      {safeReturn && (
        <div className="mb-2">
          <Link
            to={safeReturn}
            className="inline-flex min-h-11 items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground hover:bg-accent/50 hover:text-foreground transition-colors"
          >
            <ChevronLeft className="h-4 w-4 shrink-0" />
            <span>
              {t(
                safeReturn === '/chat' || safeReturn === '/studio'
                  ? 'Back to Chat'
                  : 'Back to conversation',
              )}
            </span>
          </Link>
        </div>
      )}

      {categories.map((category) => (
        <div key={category.id} className="space-y-1.5">
          <h3 className="px-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground/80">
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
                  className={`group flex items-start gap-3 rounded-lg px-3 py-2.5 transition-all min-h-11 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring ${
                    isActive
                      ? 'bg-accent text-primary font-medium shadow-sm'
                      : 'text-foreground/80 hover:bg-accent/50 hover:text-foreground'
                  }`}
                >
                  <div
                    className={`mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md transition-colors ${
                      isActive
                        ? 'bg-primary/10 text-primary'
                        : 'text-muted-foreground group-hover:text-foreground'
                    }`}
                  >
                    <Icon className="h-4 w-4" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between gap-1">
                      <span
                        className={`text-sm font-medium leading-none ${
                          isActive ? 'text-primary font-semibold' : ''
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
                    <p
                      className={`mt-1 text-xs leading-normal line-clamp-1 ${
                        isActive ? 'text-primary/75' : 'text-muted-foreground'
                      }`}
                    >
                      {t(item.subtitleKey)}
                    </p>
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
