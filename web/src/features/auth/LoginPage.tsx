import { useId, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { ApiError } from '@/api/client'
import { WordmarkWithMascot } from '@/components/brand/Wordmark'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'
import { useLogin } from './auth'

const GENERIC_ERROR = 'Не получилось войти. Попробуй ещё раз.'

export function LoginPage() {
  const navigate = useNavigate()
  const loginMutation = useLogin()
  const [localError, setLocalError] = useState<string | null>(null)
  const errorId = useId()

  const apiError = loginMutation.error
  const message =
    localError ??
    (apiError ? (apiError instanceof ApiError ? apiError.message : GENERIC_ERROR) : null)
  // Поля красим только когда ошибка про сами данные: неверный пароль или пустая форма.
  const fieldsInvalid =
    localError !== null || (apiError instanceof ApiError && apiError.code === 'invalid_credentials')

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    const login = String(form.get('login') ?? '').trim()
    const password = String(form.get('password') ?? '')
    if (!login || !password) {
      setLocalError('Введи логин и пароль')
      return
    }
    setLocalError(null)
    try {
      await loginMutation.mutateAsync({ login, password })
      navigate('/', { replace: true })
    } catch {
      // сообщение покажется из состояния мутации
    }
  }

  const clearErrors = () => {
    if (localError) setLocalError(null)
    if (loginMutation.isError) loginMutation.reset()
  }

  return (
    <main className="grid min-h-dvh place-items-center p-6">
      <div className="flex w-full max-w-md flex-col items-center gap-8">
        <WordmarkWithMascot scale={4} className="max-w-full" />

        <form
          onSubmit={onSubmit}
          noValidate
          aria-describedby={message ? errorId : undefined}
          className="pixel-box flex w-full flex-col gap-6 p-6"
        >
          <div className="flex flex-col gap-1">
            <h1 className="font-pixel text-title">Вход</h1>
            <p className="text-sm text-muted-foreground">Личный тренажёр английского.</p>
          </div>

          {message && (
            <div
              id={errorId}
              role="alert"
              className="border-4 border-coral bg-card p-3 text-sm font-semibold text-coral-text"
            >
              {message}
            </div>
          )}

          <div className="flex flex-col gap-2">
            <Label htmlFor="login">Логин</Label>
            <Input
              id="login"
              name="login"
              autoComplete="username"
              autoCapitalize="none"
              spellCheck={false}
              aria-invalid={fieldsInvalid}
              onChange={clearErrors}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="password">Пароль</Label>
            <Input
              id="password"
              name="password"
              type="password"
              autoComplete="current-password"
              aria-invalid={fieldsInvalid}
              onChange={clearErrors}
            />
          </div>

          <Button
            type="submit"
            size="lg"
            disabled={loginMutation.isPending}
            className={cn('w-full')}
          >
            {loginMutation.isPending ? 'Входим…' : 'Войти'}
          </Button>
        </form>
      </div>
    </main>
  )
}
