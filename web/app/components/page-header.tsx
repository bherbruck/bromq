import { ReactNode } from 'react'

interface PageHeaderProps {
  title: ReactNode
  description?: string
  action?: ReactNode
}

export function PageHeader({ title, description, action }: PageHeaderProps) {
  return (
    <div className="flex items-start justify-between mb-6">
      <div className="space-y-1">
        {typeof title === 'string' ? (
          <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
        ) : (
          title
        )}
        {description && <p className="text-muted-foreground">{description}</p>}
      </div>
      {action && <div className="flex items-center gap-2">{action}</div>}
    </div>
  )
}
