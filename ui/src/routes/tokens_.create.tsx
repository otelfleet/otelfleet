import { createFileRoute } from '@tanstack/react-router'
import { CreateTokenPage } from '../tokens/CreateTokenPage'

export const Route = createFileRoute('/tokens_/create')({
  component: RouteComponent,
})

function RouteComponent() {
  return <CreateTokenPage />
}
