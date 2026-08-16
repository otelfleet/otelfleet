import { useState, useCallback, type FC } from 'react'
import { Link, Outlet } from '@tanstack/react-router'
import ColorSchemeContext from '../contexts/ColorSchemeContext';
import { useColorScheme } from '../contexts/useColorScheme';
import { Notifications } from '@mantine/notifications';
import { elevationShadows, elevationStylesOverrides } from '../theme/elevation';


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
import { GitHubLogoIcon, SunIcon, MoonIcon, StackIcon, IdCardIcon, RocketIcon, MixerHorizontalIcon } from '@radix-ui/react-icons';
import { COMPONENT_ENTITY_TYPES, COLLECTOR_ENTITY_TYPES } from '../resources/entityTypes';


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
        // Navy ramp shared with the elevation scale in theme/elevation.css:
        // 0-3 are text/border tints, 4-8 are the elevation surfaces, 9 is the
        // deepest well. Keep 5-8 in sync with the --elevation-* variables.
        dark: [
            '#d3d8e3',
            '#a7b0c4',
            '#8189a0',
            '#5d6580',
            '#28344f',
            '#1f2a41',
            '#172033',
            '#0f1626',
            '#0a1020',
            '#050912',
        ],
    },
    shadows: {
        xs: '0 1px 2px rgba(0, 0, 0, 0.05)',
        sm: elevationShadows.surface,
        md: elevationShadows.raised,
        lg: elevationShadows.overlay,
        xl: '0 20px 40px rgba(0, 0, 0, 0.2), 0 8px 16px rgba(0, 0, 0, 0.1)',
    },

    headings: {
        fontFamily: 'Roboto, sans-serif',
        sizes: {
            h1: { fontSize: '36px' },
        },
    },

    components: {
        AppShell: {
            styles: elevationStylesOverrides.AppShell,
        },
        Paper: {
            styles: elevationStylesOverrides.Paper,
        },
        Card: {
            styles: elevationStylesOverrides.Card,
        },
        Modal: {
            styles: elevationStylesOverrides.Modal,
        },
        Menu: {
            styles: elevationStylesOverrides.Menu,
        },
        Combobox: {
            styles: elevationStylesOverrides.Combobox,
        },
        Popover: {
            styles: elevationStylesOverrides.Popover,
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
    const [componentsOpened, setComponentsOpened] = useState(false);
    const [colorScheme, setColorScheme] = useLocalStorage<MantineColorScheme>({
        key: 'mantine-color-scheme',
        defaultValue: 'auto',
    });

    const toggleColorScheme = useCallback(() => {
        setColorScheme((current) => {
            if (current === 'auto') return 'dark';
            if (current === 'dark') return 'light';
            return 'auto';
        });
    }, [setColorScheme]);

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
                            src={`${import.meta.env.BASE_URL}otelfleet.png`}
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
                            label="Management"
                            description="Fleet & Authorization"
                            opened={active === 'mgmt'}
                            active={active === 'mgmt'}
                            onClick={() => setActive(active === 'mgmt' ? null : 'mgmt')}
                        >
                            <NavLink component={Link} to="/tokens" label="API tokens" leftSection={<IdCardIcon />} />
                        </NavLink>

                        <NavLink
                            label="Configs"
                            description="Pipelines & exporters"
                            opened={active === 'configs'}
                            active={active === 'configs'}
                            onClick={() => setActive(active === 'configs' ? null : 'configs')}
                        >
                            {COLLECTOR_ENTITY_TYPES.map((entityType) => (
                                <NavLink
                                    key={entityType.slug}
                                    label={entityType.label}
                                    leftSection={<entityType.icon />}
                                    renderRoot={(props) => (
                                        <Link to="/resources/$type" params={{ type: entityType.slug }} {...props} />
                                    )}
                                />
                            ))}
                            <NavLink
                                label="Components"
                                description="Individual collector components"
                                leftSection={<StackIcon />}
                                opened={componentsOpened}
                                onClick={(event) => {
                                    event.preventDefault();
                                    setComponentsOpened((o) => !o);
                                }}
                            >
                                {COMPONENT_ENTITY_TYPES.map((entityType) => (
                                    <NavLink
                                        key={entityType.slug}
                                        label={entityType.label}
                                        leftSection={<entityType.icon />}
                                        renderRoot={(props) => (
                                            <Link to="/resources/$type" params={{ type: entityType.slug }} {...props} />
                                        )}
                                    />
                                ))}
                            </NavLink>
                        </NavLink>

                        <NavLink
                            label="Collectors"
                            description="Deployed collectors"
                            opened={active === 'collectors'}
                            active={active === 'collectors'}
                            onClick={() => setActive(active === 'collectors' ? null : 'collectors')}
                        >
                            <NavLink component={Link} to="/deployments" label="Deployments" leftSection={<RocketIcon />} />
                            <NavLink component={Link} to="/configfilter" label="Config Assignment" leftSection={<MixerHorizontalIcon />} />
                        </NavLink>
                    </Stack>
                </AppShell.Navbar>
                <AppShell.Main style={{ display: 'flex', flexDirection: 'column', height: '100dvh', minHeight: 0, overflow: 'auto' }}>
                    <Outlet></Outlet>
                </AppShell.Main>
            </AppShell>
            </MantineProvider>
        </ColorSchemeContext.Provider>
    )
}

export default Base