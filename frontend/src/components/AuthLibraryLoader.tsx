import * as React from 'react';
import { Redirect } from 'react-router-dom';
import axios from 'axios';

enum APIStatus {
  LOADING,
  SUCCESS,
  ERROR,
}

const AuthLibraryLoader: React.FC<{}> = () => {
  const [status, setStatus] = React.useState(APIStatus.LOADING);

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

    axios
      .get(configURL)
      .then(({data}) => {
        loadAuthLibrary(
          data['auth-client-library-url'],
          () => {
            console.log('client library load success!');
            window.clientConfig = JSON.parse(data['auth-client-config']);
            console.log('using client configuration: ' + JSON.stringify(window.clientConfig));
            window.keycloak = window.Keycloak(window.clientConfig);
            window.keycloak
              .init()
              .success(function(authenticated) {
                if (authenticated === true) {
                  axios.defaults.headers.common['Authorization'] =
                    'Bearer ' + window.keycloak.token;
                } else {
                  console.log('Not logged in!!');
                }
                setStatus(APIStatus.SUCCESS);
              })
              .error(function() {
                console.log('failed to initialize');
                setStatus(APIStatus.ERROR);
              });
          },
          () => {
            setStatus(APIStatus.ERROR);
          },
        );
      })
      .catch((error) => {
        setStatus(APIStatus.ERROR);
      });
  }, []);

  switch (status) {
    case APIStatus.LOADING:
      return <div>Loading...</div>;
    case APIStatus.SUCCESS:
      return <Redirect to="/Home" />;
    case APIStatus.ERROR:
      return <Redirect to="/Error" />;
    default:
      return null;
  }
};

export default AuthLibraryLoader;
