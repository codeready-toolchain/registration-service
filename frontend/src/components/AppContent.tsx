import * as React from 'react';
import { Route, Switch } from 'react-router-dom';
import Home from './Home';
import AuthLibraryLoader from './AuthLibraryLoader';

const AppContent: React.FC<{}> = () => {
    return (
        <Switch>
            <Route exact path="/Provision">
                <div>Provisioning Page</div>
            </Route>
            <Route exact path="/Dashboard">
                <div>Dashboard Page</div>
            </Route>
            <Route exact path="/Home">
                <Home />
            </Route>
            <Route exact path="/">
                <AuthLibraryLoader />
            </Route>
        </Switch>
    );
};

export default AppContent;
