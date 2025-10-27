import { HelpCircle } from 'lucide-react'
import { ReactNode } from 'react'
import { HoverCard, HoverCardContent, HoverCardTrigger } from '~/components/ui/hover-card'

interface PageTitleProps {
  children: string
  help?: ReactNode
}

export function PageTitle({ children, help }: PageTitleProps) {
  if (!help) {
    return <h1 className="text-2xl font-bold tracking-tight">{children}</h1>
  }

  return (
    <div className="flex items-center gap-2">
      <h1 className="text-2xl font-bold tracking-tight">{children}</h1>
      <HoverCard>
        <HoverCardTrigger asChild>
          <button className="text-muted-foreground hover:text-foreground">
            <HelpCircle className="h-4 w-4" />
          </button>
        </HoverCardTrigger>
        <HoverCardContent className="w-80">
          <div className="space-y-2 text-sm">{help}</div>
        </HoverCardContent>
      </HoverCard>
    </div>
  )
}
