import { createFileRoute } from '@tanstack/react-router'
import { ConfigFilterPage } from '../configfilters/ConfigFilterPage'

export const Route = createFileRoute('/configfilter')({
  component: RouteComponent,
})

function RouteComponent() {
  return <ConfigFilterPage />
}
