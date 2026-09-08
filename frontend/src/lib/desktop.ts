import * as DesktopService from '../../bindings/changeme/server/desktop/desktopservice'

export { DesktopService }
export type DesktopState = Awaited<ReturnType<typeof DesktopService.State>>
