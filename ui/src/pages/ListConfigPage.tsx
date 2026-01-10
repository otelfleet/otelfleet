import {
  Table,
  type ColumnConfig
} from '../components/Table'

import { useClient } from '../api';
import { ConfigService } from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import { useState, useEffect } from 'react';
import type { ConfigReference } from '../gen/api/pkg/api/config/v1alpha1/config_pb'
import AddIcon from '@mui/icons-material/Add';
import { Link } from '@tanstack/react-router'
import { Group, Button } from '@mantine/core'
import { notifyGRPCError } from '../api/notifications';

const configColumns: ColumnConfig<ConfigReference>[] = [
  { key: 'id', label: 'Name', visible: true },
]

type ConfigRow = {
  ref: ConfigReference
  actions: string[]
}


export const ConfigPage = () => {
  const client = useClient(ConfigService)

  const [configState, setState] = useState<ConfigReference[]>([])

  const handleListConfigs = async () => {
    try {
      const response = await client.listConfigs({})
      setState(response.configs)
    } catch (error) {
      notifyGRPCError("Failed to list configs", error)
    }
  }

  useEffect(() => {
    handleListConfigs()
  }, [])

  return (
    <>
      <Group style={{ marginBottom: 16, display: 'flex', justifyContent: 'flex-end' }}>

        <Link
          to="/editor"
          style={{ display: 'inline-block' }}
        >
          <Button>
            New Config
          </Button>
        </Link>
      </Group>
      <Table<ConfigReference> title="OpenTelemetry Collector configurations" data={configState} columns={configColumns} />
    </>

  )
}