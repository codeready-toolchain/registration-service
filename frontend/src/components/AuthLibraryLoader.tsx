import * as React from 'react';
import { Redirect } from 'react-router-dom';
import axios from 'axios';

enum Status {
  LOADING,
  SUCCESS,
  ERROR,
  PROVISION,
  DASHBOARD,
}

const AuthLibraryLoader: React.FC<{}> = () => {
  const [status, setStatus] = React.useState(Status.LOADING);
  const consoleURLRef = React.useRef(null);

  React.useEffect(() => {
    const configURL = '/api/v1/authconfig';

    const loadAuthLibrary = (url, cbSuccess, cbError) => {
      var script = document.createElement('script');
      script.setAttribute('src', url);
      script.setAttribute('type', 'text/javascript');
      var loaded = false;
      var loadFunction = function() {
        if (loaded) return;
        loaded = true;
        cbSuccess();
      };
      var errorFunction = function(error) {
        if (loaded) return;
        cbError(error);
      };
      script.onerror = errorFunction;
      script.onload = loadFunction;
      document.getElementsByTagName('head')[0].appendChild(script);
    };

    const getUserSignup = () => {
      return axios.get('/api/v1/signup');
    };

    axios
      .get(configURL)
      .then(({ data: configData }) => {
        loadAuthLibrary(
          configData['auth-client-library-url'],
          () => {
            window.clientConfig = JSON.parse(configData['auth-client-config']);
            window.keycloak = window.Keycloak(window.clientConfig);
            window.keycloak
              .init({
                onLoad: 'check-sso',
                silentCheckSsoRedirectUri: window.location.origin,
                promiseType: 'native',
              })
              .success((authenticated) => {
                if (authenticated === true) {
                  const action = window.sessionStorage.getItem('crtcAction');
                  axios.defaults.headers.common['Authorization'] =
                    'Bearer ' + window.keycloak.token;

                  getUserSignup()
                    .then(({ data: signupData }) => {
                      if (signupData.status.ready) {
                        consoleURLRef.current = signupData.consoleURL;
                        setStatus(Status.DASHBOARD);
                      } else {
                        setStatus(Status.PROVISION);
                      }
                    })
                    .catch(() => {
                      if (action && action === 'PROVISION') {
                        setStatus(Status.PROVISION);
                      } else {
                        setStatus(Status.SUCCESS);
                      }
                    });
                } else {
                  setStatus(Status.SUCCESS);
                }
                window.sessionStorage.removeItem('crtcAction');
              })
              .error(() => {
                setStatus(Status.ERROR);
              });
          },
          () => {
            setStatus(Status.ERROR);
          },
        );
      })
      .catch((error) => {
        setStatus(Status.ERROR);
      });
  }, []);

  switch (status) {
    case Status.LOADING:
      return <div>Loading...</div>;
    case Status.SUCCESS:
      return <Redirect to="/Home" />;
    case Status.PROVISION:
      return <Redirect to="/Provision" />;
    case Status.ERROR:
      return <Redirect to="/Error" />;
    case Status.DASHBOARD:
      return (
        <Redirect to={{ pathname: '/Dashboard', state: { consoleURL: consoleURLRef.current } }} />
      );
    default:
      return null;
  }
};

export default AuthLibraryLoader;
