import * as DesktopService from '../../bindings/changeme/server/desktopservice'

export { DesktopService }
export type DesktopState = Awaited<ReturnType<typeof DesktopService.State>>
