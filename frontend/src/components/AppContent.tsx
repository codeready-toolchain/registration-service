import * as React from 'react';
import { Route, Switch } from 'react-router-dom';
import Home from './Home';

const AppContent: React.FC<{}> = () => {
    return (
        <Switch>
            <Route exact path="/Provision">
                <div>Provisioning Page</div>
            </Route>
            <Route exact path="/Dashboard">
                <div>Dashboard Page</div>
            </Route>
            <Route exact path="/">
                <Home />
            </Route>
        </Switch>
    );
};

export default AppContent;
