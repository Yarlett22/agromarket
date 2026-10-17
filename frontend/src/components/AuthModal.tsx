import { useState } from 'react';
import { api, session, User } from '../api';

interface Props {
  onClose: () => void;
  onLogged: (u: User) => void;
}

export default function AuthModal({ onClose, onLogged }: Props) {
  const [mode, setMode] = useState<'login' | 'register'>('register');
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const res = mode === 'login' ? await api.login(email, password) : await api.register(name, email, password);
      session.save(res.token, res.user);
      onLogged(res.user);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="overlay" onClick={onClose}>
      <form className="modal" onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <h2>{mode === 'login' ? 'Inicia sesión' : 'Crea tu cuenta'}</h2>
        <p className="muted">Tu sesión usa un token JWT que valida el API Gateway.</p>
        {mode === 'register' && (
          <label>Nombre o empresa<input value={name} onChange={(e) => setName(e.target.value)} required /></label>
        )}
        <label>Email<input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required /></label>
        <label>Contraseña<input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={4} /></label>
        {error && <div className="error">{error}</div>}
        <button className="btn primary" disabled={busy}>{busy ? 'Procesando…' : mode === 'login' ? 'Entrar' : 'Registrarme'}</button>
        <button type="button" className="link" onClick={() => setMode(mode === 'login' ? 'register' : 'login')}>
          {mode === 'login' ? '¿No tienes cuenta? Regístrate' : '¿Ya tienes cuenta? Inicia sesión'}
        </button>
      </form>
    </div>
  );
}
