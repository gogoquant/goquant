import React from 'react';
import { BrowserRouter as Router } from 'react-router-dom';
import { renderRoutes } from 'react-router-config';
import { LocaleProvider } from 'antd';
import zhCN from 'antd/lib/locale-provider/zh_CN';
import { asyncRender } from 'utils';
import ScrollToTop from 'components/ScrollToTop';

const BasicLayout = asyncRender(() =>
  import(/* webpackChunkName: "basic-layout" */ './layouts/BasicLayout'),
);
const Home = asyncRender(() =>
  import(/* webpackChunkName: "home" */ './routes/Home'),
);
const Post = asyncRender(() =>
  import(/* webpackChunkName: "post" */ './routes/Post'),
);
const Topic = asyncRender(() =>
  import(/* webpackChunkName: "topic" */ './routes/Topic'),
);
const User = asyncRender(() =>
  import(/* webpackChunkName: "user" */ './routes/User'),
);
const Register = asyncRender(() =>
  import(/* webpackChunkName: "register" */ './routes/Register'),
);
const Login = asyncRender(() =>
  import(/* webpackChunkName: "login" */ './routes/Login'),
);

const routerConfig = [
  {
    component: BasicLayout,
    routes: [
      {
        path: '/',
        exact: true,
        component: Home,
      },
      {
        path: '/topic/create',
        exact: true,
        component: Post,
      },
      {
        path: '/topic/:id',
        exact: true,
        strict: false,
        component: Topic,
      },
      {
        path: '/topic/:id/edit',
        exact: true,
        strict: true,
        component: Post,
      },
      {
        path: '/user/:name',
        component: User,
      },
      {
        path: '/login',
        component: Login,
      },
      {
        path: '/register',
        component: Register,
      },
    ],
  },
];

const App = () => {
  return (
    <LocaleProvider locale={zhCN}>
      <Router>
        <ScrollToTop>{renderRoutes(routerConfig)}</ScrollToTop>
      </Router>
    </LocaleProvider>
  );
};

export default App;