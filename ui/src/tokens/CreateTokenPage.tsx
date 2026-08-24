import { useCallback } from 'react';
import { Box, Title } from '@mantine/core';
import { notifications } from '@mantine/notifications';
import { useNavigate } from '@tanstack/react-router';
import { CheckCircledIcon } from '@radix-ui/react-icons';
import { create } from '@bufbuild/protobuf';

import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';
import { CreateTokenRequestSchema, TokenService } from '../gen/api/pkg/api/bootstrap/v1alpha1/bootstrap_pb';
import { TokenForm } from './TokenForm';
import { emptyTokenValues, toLabelsMap, type TokenValues } from './token';

export function CreateTokenPage() {
  const tokenClient = useClient(TokenService);
  const navigate = useNavigate();

  const handleSubmit = useCallback(async (values: TokenValues) => {
    try {
      const token = await tokenClient.createToken(create(CreateTokenRequestSchema, {
        TTL: { seconds: BigInt(values.ttlSeconds) },
        labels: toLabelsMap(values.labels),
      }));
      notifications.show({
        title: 'Token successfully created',
        message: 'Bootstrap token successfully created',
        icon: <CheckCircledIcon />,
      });
      navigate({ to: '/tokens/$tokenId', params: { tokenId: token.ID } });
    } catch (error) {
      notifyGRPCError('Create token error', error);
    }
  }, [tokenClient, navigate]);

  return (
    <Box>
      <Title order={3} mb="md">Create Token</Title>
      <TokenForm
        initialValues={emptyTokenValues()}
        onSubmit={handleSubmit}
        onCancel={() => navigate({ to: '/tokens' })}
      />
    </Box>
  );
}
