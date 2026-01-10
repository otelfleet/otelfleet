import { useState, type FC } from 'react'
import { Link, Outlet } from '@tanstack/react-router'
import ColorSchemeContext, { useColorScheme } from '../contexts/ColorSchemeContext';
import { Notifications } from '@mantine/notifications';


import {
    createTheme,
    MantineProvider,
    AppShell,
    Burger,
    Stack,
    NavLink,
    Group,
    ActionIcon,
    useComputedColorScheme,
    type MantineColorScheme,
} from '@mantine/core';
import { useDisclosure, useLocalStorage } from '@mantine/hooks'
import { GitHubLogoIcon, SunIcon, MoonIcon } from '@radix-ui/react-icons';


const theme = createTheme({
    colors: {
        // Add your color
        deepBlue: [
            '#eef3ff',
            '#dce4f5',
            '#b9c7e2',
            '#94a8d0',
            '#748dc1',
            '#5f7cb8',
            '#5474b4',
            '#44639f',
            '#39588f',
            '#2d4b81',
        ],
        // or replace default theme color
        blue: [
            '#eef3ff',
            '#dee2f2',
            '#bdc2de',
            '#98a0ca',
            '#7a84ba',
            '#6672b0',
            '#5c68ac',
            '#4c5897',
            '#424e88',
            '#364379',
        ],
        dark: [
            '#d5d7e0',
            '#acaebf',
            '#8c8fa3',
            '#666980',
            '#4d4f66',
            '#34354a',
            '#2b2c3d',
            '#1d1e30',
            '#0c0d21',
            '#01010a',
        ],
    },
    shadows: {
        md: '1px 1px 3px rgba(0, 0, 0, .25)',
        xl: '5px 5px 3px rgba(0, 0, 0, .25)',
    },

    headings: {
        fontFamily: 'Roboto, sans-serif',
        sizes: {
            h1: { fontSize: '36px' },
        },
    },
});




const ColorSchemeToggle: FC = () => {
    const { toggleColorScheme } = useColorScheme();
    const computedColorScheme = useComputedColorScheme('light');

    return (
        <ActionIcon
            onClick={toggleColorScheme}
            variant="default"
            size="lg"
            aria-label="Toggle color scheme"
        >
            {computedColorScheme === 'light' ? <MoonIcon /> : <SunIcon />}
        </ActionIcon>
    );
};

const Base: FC = () => {
    const [opened, { toggle }] = useDisclosure();
    const [active, setActive] = useState<string | null>(null);
    const [colorScheme, setColorScheme] = useLocalStorage<MantineColorScheme>({
        key: 'mantine-color-scheme',
        defaultValue: 'auto',
    });

    const toggleColorScheme = () => {
        setColorScheme((current) => {
            if (current === 'auto') return 'dark';
            if (current === 'dark') return 'light';
            return 'dark';
        });
    };

    return (
        <ColorSchemeContext.Provider value={{ colorScheme, setColorScheme, toggleColorScheme }}>
            <MantineProvider theme={theme} defaultColorScheme="auto" forceColorScheme={colorScheme === 'auto' ? undefined : colorScheme}>
                <Notifications />
            <AppShell
                padding="md"
                header={{ height: 60 }}
                navbar={{
                    width: 300,
                    breakpoint: 'sm',
                    collapsed: { mobile: !opened }
                }}
            >
                <AppShell.Header>
                    <Burger
                        opened={opened}
                        onClick={toggle}
                        hiddenFrom="sm"
                        size="sm"
                    ></Burger>

                    <Group justify="space-between" style={{ flex: 1, height: '100%', alignItems: 'center', paddingLeft: 12, paddingRight: 12 }}>
                        <img
                            src="/otelfleet.png"
                            alt="otelfleet logo"
                            style={{ height: '90%', maxHeight: '100%', objectFit: 'contain' }}
                        />
                        <Group gap="sm">
                            <ColorSchemeToggle />
                            <a href="https://github.com/otelfleet" target="_blank" rel="noopener noreferrer" style={{ display: 'flex', alignItems: 'center', color: 'inherit' }}>
                                <GitHubLogoIcon style={{ height: '90%', maxHeight: '100%' }} />
                            </a>
                        </Group>
                    </Group>

                </AppShell.Header>
                <AppShell.Navbar>
                    <Stack gap="xs">
                        <NavLink
                            label="Tokens"
                            description="Manage API tokens"
                            opened={active === 'tokens'}
                            active={active === 'tokens'}
                            onClick={() => setActive(active === 'tokens' ? null : 'tokens')}
                        >
                            <Link to="/tokens" style={{ all: 'unset', display: 'inline-block', cursor: 'pointer' }}>
                                <NavLink label="All tokens" onClick={() => console.log('tokens/all')} />
                            </Link>
                        </NavLink>

                        <NavLink
                            label="Configs"
                            description="Pipelines & exporters"
                            opened={active === 'configs'}
                            active={active === 'configs'}
                            onClick={() => setActive(active === 'configs' ? null : 'configs')}
                        >
                            <Link to="/configs" style={{ all: 'unset', display: 'inline-block', cursor: 'pointer' }}>
                                <NavLink label="All configs" onClick={() => console.log('configs/all')} />
                            </Link>
                        </NavLink>

                        <NavLink
                            label="Agents"
                            description="Deployed collectors"
                            opened={active === 'agents'}
                            active={active === 'agents'}
                            onClick={() => setActive(active === 'agents' ? null : 'agents')}
                        >
                            <Link to="/agents" style={{ all: 'unset', display: 'inline-block', cursor: 'pointer' }}>
                                <NavLink label="All agents" onClick={() => console.log('agents/all')} />
                            </Link>
                    
                        </NavLink>
                    </Stack>
                </AppShell.Navbar>
                <AppShell.Main>
                    <Outlet></Outlet>
                </AppShell.Main>
            </AppShell>
            </MantineProvider>
        </ColorSchemeContext.Provider>
    )
}

export default Base