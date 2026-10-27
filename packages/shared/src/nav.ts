/**
 * The "previous menu" of a panel path, used when there is no history to go back to
 * (web reload or a direct link): /admin/b/catalog/x -> /admin/b/catalog -> /admin/b -> /admin.
 * null at the panel root.
 */
export function parentPath(pathname: string, root = '/admin'): string | null {
  const clean = pathname.replace(/\/+$/, '');
  if (!clean.startsWith(root + '/')) return null;
  return clean.slice(0, clean.lastIndexOf('/'));
}
