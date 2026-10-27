// Suggests a URL slug from a business name: "Uñas Bellas, Chihuahua" -> "unas-bellas-chihuahua".
// The API validates again; this only pre-fills the field.
export function slugify(name: string): string {
  return name
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '') // quita acentos y la tilde de la ñ
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 60)
    .replace(/-+$/, '');
}
