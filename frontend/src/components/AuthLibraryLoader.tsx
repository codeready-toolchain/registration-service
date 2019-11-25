import * as React from 'react';
import { Redirect } from 'react-router-dom';
import axios from 'axios';

enum Status {
  LOADING,
  SUCCESS,
  ERROR,
  PROVISION,
}

const AuthLibraryLoader: React.FC<{}> = () => {
  const [status, setStatus] = React.useState(Status.LOADING);

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
      .then(({ data }) => {
        loadAuthLibrary(
          data['auth-client-library-url'],
          () => {
            console.log('client library load success!');
            window.clientConfig = JSON.parse(data['auth-client-config']);
            console.log('using client configuration: ' + JSON.stringify(window.clientConfig));
            window.keycloak = window.Keycloak(window.clientConfig);
            window.keycloak
              .init({
                onLoad: 'check-sso',
                silentCheckSsoRedirectUri: window.location.origin,
                promiseType: 'native',
            })
              .success((authenticated) => {
                if (authenticated === true) {
                  console.log('Logged in!!');
                  const action = window.sessionStorage.getItem('crtcAction');
                  axios.defaults.headers.common['Authorization'] =
                    'Bearer ' + window.keycloak.token;

                  if (action && action === 'PROVISION') {
                    window.sessionStorage.removeItem('crtcentAction');
                    setStatus(Status.PROVISION);
                    return;
                  }

                  getUserSignup()
                    .then(({ data }) => {
                      setStatus(Status.PROVISION);
                    }).catch(() => {
                      setStatus(Status.SUCCESS);
                      console.log("CodeReady Toolchain account is not provisioned.");
                    });
                } else {
                  console.log('Not logged in!!');
                  setStatus(Status.SUCCESS);
                }
              })
              .error(() => {
                console.log('failed to initialize');
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
    default:
      return null;
  }
};

export default AuthLibraryLoader;
