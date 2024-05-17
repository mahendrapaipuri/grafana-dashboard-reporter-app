import {
  NAVIGATION,
  NAVIGATION_TITLE,
  NAVIGATION_SUBTITLE,
  PLUGIN_BASE_URL,
} from "../constants";

// Prefixes the route with the base URL of the plugin
export function prefixRoute(route: string): string {
  return `${PLUGIN_BASE_URL}/${route}`;
}

export function getNavModel({
  activeId,
  basePath,
  logoUrl,
}: {
  activeId: string;
  basePath: string;
  logoUrl: string;
}) {
  const main = {
    text: NAVIGATION_TITLE,
    subTitle: NAVIGATION_SUBTITLE,
    url: basePath,
    img: logoUrl,
    children: Object.values(NAVIGATION).map((navItem) => ({
      ...navItem,
      active: navItem.id === activeId,
    })),
  };

  return {
    main,
    node: main,
  };
}
