import * as _ from 'lodash-es';
import { createBrowserHistory, createMemoryHistory, History } from 'history';

type AppHistory = History & { pushPath: History['push'] };

let createHistory;

try {
  // eslint-disable-next-line no-undef
  if (process.env.NODE_ENV === 'test') {
    // Running in node. Can't use browser history
    createHistory = createMemoryHistory;
  } else {
    createHistory = createBrowserHistory;
  }
} catch (unused) {
  createHistory = createBrowserHistory;
}

export const history: AppHistory = createHistory({ basename: '/' });

const removeBasePath = (url = '/') =>
  _.startsWith(url, '/')
    ? url.slice('/'.length - 1)
    : url;

// Monkey patch history to slice off the base path
(history as any).__replace__ = history.replace;
history.replace = (url) => (history as any).__replace__(removeBasePath(url));

(history as any).__push__ = history.push;
history.push = (url) => (history as any).__push__(removeBasePath(url));
(history as any).pushPath = (path) => (history as any).__push__(path);

