import { createRoot } from 'react-dom/client';
import { CssBaseline } from '@mui/material';
import { BrowserRouter } from 'react-router';
import App from './App';
import { AuthProvider } from './features/auth/AuthProvider';

createRoot(document.getElementById('root')!).render(
  <BrowserRouter><CssBaseline /><AuthProvider><App /></AuthProvider></BrowserRouter>,
);
