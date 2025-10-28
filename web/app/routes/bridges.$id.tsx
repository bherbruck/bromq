import type { Route } from './+types/bridges.$id'
import { BridgeDetail } from '~/components/bridge-detail'

export const meta: Route.MetaFunction = () => [{ title: 'Bridge Details - MQTT Server' }]

export default function BridgeDetailPage({ params }: Route.ComponentProps) {
  const bridgeId = parseInt(params.id, 10)

  if (isNaN(bridgeId)) {
    return (
      <div className="flex flex-col items-center justify-center p-12">
        <p className="text-muted-foreground text-sm">Invalid bridge ID</p>
      </div>
    )
  }

  return <BridgeDetail bridgeId={bridgeId} />
}
