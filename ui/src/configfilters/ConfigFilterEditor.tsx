import { useCallback, useEffect, useState } from 'react';
import { Box, Loader, Title } from '@mantine/core';
import { notifications } from '@mantine/notifications';
import { useNavigate } from '@tanstack/react-router';
import { CheckCircledIcon } from '@radix-ui/react-icons';

import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';
import { ResourceService } from '../gen/api/pkg/api/resources/v1alpha1/resources_pb';
import { getEntityType } from '../resources/entityTypes';
import { ConfigFilterForm } from './ConfigFilterForm';
import {
  CONFIG_FILTER_TYPE_URL,
  emptyConfigFilterValues,
  packConfigFilter,
  toConfigFilterValues,
  unpackConfigFilter,
  type ConfigFilterValues,
} from './configFilter';

interface ConfigFilterEditorProps {
  entityKey?: string;
}

export function ConfigFilterEditor({ entityKey }: ConfigFilterEditorProps) {
  const client = useClient(ResourceService);
  const navigate = useNavigate();
  const collectorConfigType = getEntityType('collectorconfig')!;
  const isEditMode = Boolean(entityKey);

  const [values, setValues] = useState<ConfigFilterValues | null>(isEditMode ? null : emptyConfigFilterValues());
  const [collectorConfigs, setCollectorConfigs] = useState<string[]>([]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const response = await client.listEntity({ typeUrl: collectorConfigType.typeUrl });
        if (!cancelled) setCollectorConfigs(response.entities.map((e) => e.key));
      } catch (error) {
        notifyGRPCError('Failed to list collector configs', error);
      }
    })();
    return () => { cancelled = true; };
  }, [client, collectorConfigType]);

  useEffect(() => {
    if (!entityKey) return;
    let cancelled = false;
    (async () => {
      try {
        const response = await client.getEntity({ typeUrl: CONFIG_FILTER_TYPE_URL, key: entityKey });
        if (!cancelled) setValues(toConfigFilterValues(entityKey, unpackConfigFilter(response.entity?.obj)));
      } catch (error) {
        notifyGRPCError('Failed to load config assignment', error);
      }
    })();
    return () => { cancelled = true; };
  }, [client, entityKey]);

  const handleSubmit = useCallback(async (submitted: ConfigFilterValues) => {
    try {
      await client.putEntity({
        entity: {
          typeUrl: CONFIG_FILTER_TYPE_URL,
          key: submitted.name,
          obj: packConfigFilter(submitted),
        },
      });
      notifications.show({
        title: isEditMode ? 'Config assignment updated' : 'Config assignment created',
        message: `${isEditMode ? 'Updated' : 'Created'} "${submitted.name}"`,
        icon: <CheckCircledIcon />,
      });
      navigate({ to: '/configfilter' });
    } catch (error) {
      notifyGRPCError('Failed to save config assignment', error);
    }
  }, [client, isEditMode, navigate]);

  return (
    <Box>
      <Title order={3} mb="md">{isEditMode ? `Edit ${entityKey}` : 'Config Filter'}</Title>
      {values ? (
        <ConfigFilterForm
          initialValues={values}
          collectorConfigs={collectorConfigs}
          editing={isEditMode}
          onSubmit={handleSubmit}
          onCancel={() => navigate({ to: '/configfilter' })}
        />
      ) : (
        <Loader />
      )}
    </Box>
  );
}
