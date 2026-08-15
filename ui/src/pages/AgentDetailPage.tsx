import { useEffect, useState, useCallback } from 'react';
import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';
import { AgentService } from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import type {
    AgentDescription,
    AgentStatus,
    EffectiveConfig,
} from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import { Loader, Center, Alert } from '@mantine/core';
import { AlertCircle } from 'react-feather';
import { AgentDetailView, isAgentTab, type AgentTab } from '../components/agents/agentDetail';
import { useLocation, useNavigate } from '@tanstack/react-router';

interface AgentDetailPageProps {
    agentId: string;
}

export function AgentDetailPage({ agentId }: AgentDetailPageProps) {
    const navigate = useNavigate();
    const hash = useLocation({ select: (location) => location.hash });
    const tab: AgentTab = isAgentTab(hash) ? hash : 'health';

    const handleTabChange = useCallback((next: AgentTab) => {
        navigate({ to: '.', hash: next, replace: true });
    }, [navigate]);

    const agentClient = useClient(AgentService);
    const [agent, setAgent] = useState<AgentDescription | null>(null);
    const [status, setStatus] = useState<AgentStatus | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [history, setHistory] = useState<EffectiveConfig[]>([]);
    const [historyLoading, setHistoryLoading] = useState(true);

    const fetchHistory = useCallback(async () => {
        setHistoryLoading(true);
        try {
            const response = await agentClient.agentHistory({ agentId, offset: 0n, limit: 50n });
            setHistory(response.effectiveConfig);
        } catch (err) {
            notifyGRPCError('Failed to load agent history', err);
            setHistory([]);
        } finally {
            setHistoryLoading(false);
        }
    }, [agentId, agentClient]);

    useEffect(() => {
        const fetchAgentData = async () => {
            setLoading(true);
            setError(null);
            try {
                const [agentResponse, statusResponse] = await Promise.all([
                    agentClient.getAgent({ agentId }),
                    agentClient.status({ agentId }),
                ]);
                setAgent(agentResponse.agent ?? null);
                setStatus(statusResponse.status ?? null);
            } catch (err) {
                notifyGRPCError('Failed to fetch agent details', err);
                setError('Failed to load agent details');
            } finally {
                setLoading(false);
            }
        };

        fetchAgentData();
        fetchHistory();
    }, [agentId, agentClient, fetchHistory]);

    if (loading) {
        return (
            <Center style={{ height: '100%', minHeight: 400 }}>
                <Loader size="lg" />
            </Center>
        );
    }

    if (error) {
        return (
            <Alert color="red" title="Error" icon={<AlertCircle size={16} />}>
                {error}
            </Alert>
        );
    }

    return (
        <AgentDetailView
            agent={agent}
            status={status}
            history={history}
            historyLoading={historyLoading}
            tab={tab}
            onTabChange={handleTabChange}
        />
    );
}
