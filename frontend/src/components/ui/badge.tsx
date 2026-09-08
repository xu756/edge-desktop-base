import { cn } from '@/lib/utils'
import type * as React from 'react'

function Badge({ className, ...props }: React.ComponentProps<'span'>) {
  return (
    <span
      data-slot='badge'
      className={cn(
        'bg-secondary text-secondary-foreground inline-flex h-6 items-center rounded-full px-2.5 text-xs font-medium',
        className,
      )}
      {...props}
    />
  )
}

export { Badge }
