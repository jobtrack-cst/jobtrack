import { createRoot } from 'react-dom/client';
import { CssBaseline, ThemeProvider } from '@mui/material';
import { BrowserRouter } from 'react-router';
import App from './App';
import { AuthProvider } from './features/auth/AuthProvider';
import { theme } from './theme';

createRoot(document.getElementById('root')!).render(
  <ThemeProvider theme={theme}><CssBaseline /><BrowserRouter><AuthProvider><App /></AuthProvider></BrowserRouter></ThemeProvider>,
);
