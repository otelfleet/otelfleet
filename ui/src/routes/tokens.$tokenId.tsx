import { createFileRoute } from '@tanstack/react-router'
import { TokenDetailPage } from '../pages/TokenDetailPage'

export const Route = createFileRoute('/tokens/$tokenId')({
  component: RouteComponent,
})

function RouteComponent() {
  const { tokenId } = Route.useParams()
  return <TokenDetailPage tokenId={tokenId} />
}
