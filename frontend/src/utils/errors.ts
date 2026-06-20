/**
 * Извлекает человекочитаемое сообщение из ответа сервера.
 * Пытается распарсить JSON, иначе возвращает текст как есть.
 */
export async function extractErrorMessage(res: Response): Promise<string> {
  try {
    const data = await res.json();
    if (data && typeof data.error === 'string' && data.error) {
      return data.error;
    }
    return `Ошибка сервера (${res.status})`;
  } catch {
    const text = await res.text().catch(() => '');
    if (text && text.length < 200) return text;
    return `Ошибка сервера (${res.status}). Попробуйте позже.`;
  }
}

/**
 * Универсальная обёртка для fetch с обработкой ошибок.
 * В случае неуспешного ответа выбрасывает Error с человекочитаемым сообщением.
 */
export async function apiFetch(url: string, options?: RequestInit): Promise<Response> {
  const res = await fetch(url, options);
  if (!res.ok) {
    const msg = await extractErrorMessage(res);
    throw new Error(msg);
  }
  return res;
}