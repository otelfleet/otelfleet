import { createFileRoute, notFound } from '@tanstack/react-router'
import { ResourceListPage } from '../resources/ResourceListPage'
import { getEntityType } from '../resources/entityTypes'

export const Route = createFileRoute('/resources/$type')({
  component: RouteComponent,
})

function RouteComponent() {
  const { type } = Route.useParams()
  const entityType = getEntityType(type)
  if (!entityType) throw notFound()
  return <ResourceListPage entityType={entityType} />
}
