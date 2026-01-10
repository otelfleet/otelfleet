import { createFileRoute } from '@tanstack/react-router'
import { ConfigPage } from '../pages/ListConfigPage'

export const Route = createFileRoute('/configs')({
  component: RouteComponent,
})

function RouteComponent() {
  return <ConfigPage></ConfigPage>
}
