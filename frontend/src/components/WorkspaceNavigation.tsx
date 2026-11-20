import { Box, Button, Drawer, IconButton, Stack, SvgIcon, Typography, useMediaQuery, useTheme } from '@mui/material';
import { NavLink } from 'react-router';
import { Brand } from './Brand';
import { WorkspaceIcon } from './WorkspaceIcon';

interface Props {
  base: string;
  roleName: string;
  items: [string, string][];
  open: boolean;
  onClose: () => void;
}

export function WorkspaceNavigation({ base, roleName, items, open, onClose }: Props) {
  const theme = useTheme();
  const desktop = useMediaQuery(theme.breakpoints.up('md'));
  const content = <Stack sx={{ p: 2, gap: 2, minWidth: 0 }}>
    {!desktop && <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ bgcolor: '#fffefa', borderRadius: '10px', p: 1 }}>
      <Box onClick={onClose}><Brand to={base} /></Box>
      <IconButton aria-label="Close navigation" onClick={onClose}><SvgIcon><path d="m6 6 12 12M6 18 18 6" stroke="currentColor" strokeWidth="1.7" /></SvgIcon></IconButton>
    </Stack>}
    <Typography variant="overline" sx={{ color: '#b9ccbd', px: 1, pt: .5, textTransform: 'uppercase' }}>{roleName} workspace</Typography>
    <Stack component="nav" id="workspace-navigation" aria-label="Workspace" spacing={.5}>
      {items.map(([path, label]) => <Button key={path} component={NavLink} to={`${base}/${path}`} onClick={onClose} startIcon={<WorkspaceIcon name={path} />}
        sx={{ justifyContent: 'flex-start', textAlign: 'left', overflowWrap: 'anywhere', px: 1.5, fontSize: '.85rem', fontWeight: 500,
          color: '#d8e3d7', minHeight: 46, border: '1px solid transparent',
          '& .MuiButton-startIcon': { mr: 1.5, ml: 0 },
          '&:hover': { bgcolor: '#2b5346', borderColor: '#416852', transform: 'none' },
          '&.Mui-focusVisible': { outlineColor: '#ecd2a1' },
          '&[aria-current="page"]': { bgcolor: '#edf1df', color: '#214c3c', fontWeight: 700, borderColor: '#e1dfbe',
            boxShadow: 'inset 3px 0 0 #b28c4b, 0 2px 4px rgb(0 0 0 / 6%)' } }}>{label}</Button>)}
    </Stack>
    <Box sx={{ mt: 1, px: 1.5, pt: 2.5, pb: 3.5, borderTop: '1px solid #42604f', backgroundImage: 'url(/branding/weave.svg)',
      backgroundRepeat: 'repeat-x', backgroundPosition: '0 bottom' }}>
      <Typography variant="body2" sx={{ color: '#e0e6cd', fontWeight: 600 }}>Rooted here. Ready for what’s next.</Typography>
      <Typography variant="caption" sx={{ color: '#b7cdbb', display: 'block', mt: 1 }}>College of Science and Technology</Typography>
    </Box>
  </Stack>;
  if (desktop) return <Box component="aside" aria-label="Workspace sidebar" sx={{ position: 'sticky', top: 98, minWidth: 0,
    maxHeight: 'calc(100dvh - 118px)', overflowY: 'auto', scrollbarWidth: 'thin', scrollbarColor: '#6a8971 #153e37',
    borderRadius: '14px', bgcolor: 'primary.dark', border: '1px solid #2e5345', boxShadow: '0 4px 16px rgb(21 62 55 / 10%)' }}>{content}</Box>;
  return <Drawer open={open} onClose={onClose} transitionDuration={180}
    slotProps={{ paper: { role: 'dialog', 'aria-modal': true, 'aria-label': `${roleName} navigation`,
      sx: { width: 'min(320px, calc(100vw - 24px))', bgcolor: 'primary.dark', color: '#fffefa', borderRight: '1px solid #42604f' } } }}>{content}</Drawer>;
}
